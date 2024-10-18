package bot

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	URL "net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/XiaoMengXinX/Music163Api-Go/api"
	"github.com/XiaoMengXinX/Music163Api-Go/types"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

type ReduxState struct {
	Songs []Song `json:"songs"`
}

type Song struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// 判断数组包含关系
func in(target string, strArray []string) bool {
	sort.Strings(strArray)
	index := sort.SearchStrings(strArray, target)
	if index < len(strArray) && strArray[index] == target {
		return true
	}
	return false
}

// 解析作曲家信息
func parseArtist(songDetail types.SongDetailData) string {
	var artists string
	for i, ar := range songDetail.Ar {
		if i == 0 {
			artists = ar.Name
		} else {
			artists = fmt.Sprintf("%s/%s", artists, ar.Name)
		}
	}
	return artists
}

// 判断文件夹是否存在/新建文件夹
func dirExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			logrus.Errorf("mkdir %v failed: %v\n", path, err)
		}
		return false
	}
	logrus.Errorf("Error: %v\n", err)
	return false
}

// 校验 md5
func verifyMD5(filePath string, md5str string) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer f.Close()
	md5hash := md5.New()
	if _, err := io.Copy(md5hash, f); err != nil {
		return false, err
	}
	if hex.EncodeToString(md5hash.Sum(nil)) != md5str {
		return false, errors.New(md5VerFailed)
	}
	return true, nil
}

// 解析 MusicID
func parseMusicID(text string) []int {
	var (
		err      error
		musicids []int
		replacer = strings.NewReplacer("\n", "", " ", "")
	)
	messageText := replacer.Replace(text)
	musicUrl := regUrl.FindStringSubmatch(messageText)
	if len(musicUrl) != 0 {
		client := http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		if strings.Contains(musicUrl[0], "163cn.tv") {
			resp, err := client.Get(musicUrl[0])
			if err != nil {
				return []int{0}
			}
			defer resp.Body.Close()
			musicUrl[0] = resp.Header.Get("Location")
		}

		// 歌曲
		if strings.Contains(musicUrl[0], "song") {
			url, _ := URL.Parse(musicUrl[0])
			id := url.Query().Get("id")
			if musicid, _ := strconv.Atoi(id); musicid != 0 {
				return append(musicids, musicid)
			}
		}

		// 专辑
		if strings.Contains(musicUrl[0], "album") {
			url, _ := URL.Parse(musicUrl[0])
			var id int
			if id, err = strconv.Atoi(strings.Split(url.Path, "/")[2]); err != nil {
				if id, err = strconv.Atoi(url.Query().Get("id")); err != nil {
					return []int{0}
				}
			}
			return getAlbumToNusicID(id)
		}
	}

	if musicID, err := strconv.Atoi(linkTestMusic(messageText)); err == nil {
		musicids = append(musicids, musicID)
	}
	return musicids
}

// 解析 ProgramID
func parseProgramID(text string) int {
	var replacer = strings.NewReplacer("\n", "", " ", "")
	messageText := replacer.Replace(text)
	programid, _ := strconv.Atoi(linkTestProgram(messageText))
	return programid
}

// 提取数字
func extractInt(text string) string {
	matchArr := regInt.FindStringSubmatch(text)
	if len(matchArr) == 0 {
		return ""
	}
	return matchArr[0]
}

// 解析分享链接
func linkTestMusic(text string) string {
	return extractInt(reg5.ReplaceAllString(reg4.ReplaceAllString(reg3.ReplaceAllString(reg2.ReplaceAllString(reg1.ReplaceAllString(text, ""), ""), ""), ""), ""))
}

func linkTestProgram(text string) string {
	return extractInt(reg5.ReplaceAllString(reg4.ReplaceAllString(reg3.ReplaceAllString(regP4.ReplaceAllString(regP3.ReplaceAllString(regP2.ReplaceAllString(regP1.ReplaceAllString(text, ""), ""), ""), ""), ""), ""), ""))
}

// 判断 error 是否为超时错误
// func isTimeout(err error) bool {
// 	if strings.Contains(fmt.Sprintf("%v", err), "context deadline exceeded") {
// 		return true
// 	}
// 	return false
// }

// 获取电台节目的 MusicID
func getProgramRealID(programID int) int {
	programDetail, err := api.GetProgramDetail(data, programID)
	if err != nil {
		return 0
	}
	if programDetail.Program.MainSong.ID != 0 {
		return programDetail.Program.MainSong.ID
	}
	return 0
}

func getAlbumToNusicID(albumID int) (musicids []int) {
	albumDetail, err := api.GetAlbumDetail(data, albumID)
	if err != nil {
		return []int{0}
	}

	var state ReduxState
	err = json.Unmarshal([]byte(albumDetail.RawJson), &state)
	if err != nil {
		logrus.Errorf("Error parsing JSON: %s", err)
		return []int{0}
	}

	for _, song := range state.Songs {
		musicids = append(musicids, int(song.ID))
	}
	return musicids
}

// 读取白名单列表
func readWhitelist() []int64 {
	data, _ := os.ReadFile("Whitelist")
	lines := strings.Split(string(data), "\n")
	var whitelist []int64
	for _, line := range lines {
		num, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			continue
		}
		whitelist = append(whitelist, num)
	}
	return whitelist
}

// 更新白名单列表
func AddWhitelist(message tgbotapi.Message, whitelist []string) error {
	data, _ := os.ReadFile("Whitelist")
	lines := strings.Split(string(data), "\n")
	NewList := append(lines, whitelist...)
	NewList = removeDuplicates(NewList)
	NewData := []byte(strings.Join(NewList, "\n"))
	os.WriteFile("Whitelist", NewData, 0644)

	newMsg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("增加: %s", whitelist))
	newMsg.ParseMode = tgbotapi.ModeMarkdown
	newMsg.ReplyToMessageID = message.MessageID
	message, err := bot.Send(newMsg)
	if err != nil {
		return err
	}
	return nil
}

func removeDuplicates(arr []string) []string {
	seen := make(map[string]bool)
	unique := []string{}
	for _, val := range arr {
		if !seen[val] {
			seen[val] = true
			unique = append(unique, val)
		}
	}

	return unique
}
