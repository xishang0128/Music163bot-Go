package bot

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/XiaoMengXinX/Music163Api-Go/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

var WhitelistIDs []string

// Start bot entry
func Start(conf map[string]string) (actionCode int) {
	config = conf
	defer func() {
		e := recover()
		if e != nil {
			logrus.Errorln(e)
			actionCode = 1
		}
	}()
	// 创建缓存文件夹
	dirExists(cacheDir)

	// 解析 bot 管理员配置
	botAdminStr = strings.Split(config["BotAdmin"], ",")
	if len(botAdminStr) == 0 && config["BotAdmin"] != "" {
		botAdminStr = []string{config["BotAdmin"]}
	}
	if len(botAdminStr) != 0 {
		for _, s := range botAdminStr {
			id, err := strconv.Atoi(s)
			if err == nil {
				botAdmin = append(botAdmin, id)
			}
		}
	}
	if config["Whitelist"] != "" {
		WhitelistIDs = strings.Split(config["Whitelist"], ",")
	}

	// 初始化数据库
	err := initDB(config)
	if err != nil {
		logrus.Errorln(err)
		return 1
	}

	if config["MUSIC_U"] != "" {
		data = utils.RequestData{
			Cookies: []*http.Cookie{
				{
					Name:  "MUSIC_U",
					Value: config["MUSIC_U"],
				},
			},
		}
	}
	if config["BotAPI"] != "" {
		botAPI = config["BotAPI"]
	}

	if maxRetryTimes, _ = strconv.Atoi(config["MaxRetryTimes"]); maxRetryTimes <= 0 {
		maxRetryTimes = 3
	}
	if downloaderTimeout, _ = strconv.Atoi(config["DownloadTimeout"]); downloaderTimeout <= 0 {
		downloaderTimeout = 60
	}

	// 设置 bot 日志接口
	err = tgbotapi.SetLogger(logrus.StandardLogger())
	if err != nil {
		logrus.Errorln(err)
		return 1
	}
	// 配置 token、api、debug
	bot, err = tgbotapi.NewBotAPIWithAPIEndpoint(config["BOT_TOKEN"], botAPI+"/bot%s/%s")
	if err != nil {
		logrus.Errorln(err)
		return 1
	}
	if config["BotDebug"] == "true" {
		bot.Debug = true
	}

	logrus.Printf("%s 验证成功", bot.Self.UserName)
	botName = bot.Self.UserName

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)
	defer bot.StopReceivingUpdates()

	for update := range updates {
		if update.Message == nil && update.CallbackQuery == nil && update.InlineQuery == nil { // ignore any non-Message Updates
			continue
		}
		switch {
		case update.Message != nil:
			updateMsg := *update.Message
			ChatID := strconv.FormatInt(updateMsg.Chat.ID, 10)
			found := false
			for _, id := range WhitelistIDs {
				if ChatID == id {
					found = true
					break
				}
			}
			if found || len(WhitelistIDs) == 0 {
				if atStr := strings.ReplaceAll(update.Message.CommandWithAt(), update.Message.Command(), ""); update.Message.Command() != "" && (atStr == "" || atStr == "@"+botName) {
					switch update.Message.Command() {
					case "start":
						if !updateMsg.Chat.IsPrivate() {
							return
						}
						go func() {
							musicID, _ := strconv.Atoi(updateMsg.CommandArguments())
							if musicID == 0 {
								return
							}
							err := processMusic(musicID, updateMsg, bot)
							if err != nil {
								logrus.Errorln(err)
							}
						}()
					case "music", "netease":
						go func() {
							err := processAnyMusic(updateMsg, bot)
							if err != nil {
								logrus.Errorln(err)
							}
						}()
					case "program":
						go func() {
							id, _ := strconv.Atoi(updateMsg.CommandArguments())
							musicID := getProgramRealID(id)
							if musicID != 0 {
								err := processMusic(musicID, updateMsg, bot)
								if err != nil {
									logrus.Errorln(err)
								}
							}
						}()
					case "lyric":
						go func() {
							err := processLyric(updateMsg, bot)
							if err != nil {
								logrus.Errorln(err)
							}
						}()
					case "search":
						go func() {
							err := processSearch(updateMsg, bot)
							if err != nil {
								logrus.Errorln(err)
							}
						}()
					case "recognize":
						go func() {
							err := recognizeMusic(updateMsg, bot)
							if err != nil {
								logrus.Errorln(err)
							}
						}()
					case "about":
						go func() {
							err := printAbout(updateMsg, bot)
							if err != nil {
								logrus.Errorln(err)
							}
						}()
					case "status":
						go func() {
							err := processStatus(updateMsg, bot)
							if err != nil {
								logrus.Errorln(err)
							}
						}()
					}
					if in(fmt.Sprintf("%d", update.Message.From.ID), botAdminStr) {
						switch update.Message.Command() {
						case "rmcache":
							go func() {
								err := processRmCache(updateMsg, bot)
								if err != nil {
									logrus.Errorln(err)
								}
							}()
						case "reload":
							msg := tgbotapi.NewMessage(update.Message.Chat.ID, reloading)
							msg.ReplyToMessageID = update.Message.MessageID
							_, _ = bot.Send(msg)
							return 2
						}
					}
				} else if strings.Contains(update.Message.Text, "music.163.com") || strings.Contains(update.Message.Text, "163cn.tv") {
					go func() {
						id := parseMusicID(updateMsg.Text)
						if id != 0 {
							err := processMusic(id, updateMsg, bot)
							if err != nil {
								logrus.Errorln(err)
							}
						} else if id = parseProgramID(updateMsg.Text); id != 0 {
							if id = getProgramRealID(id); id != 0 {
								err := processMusic(id, updateMsg, bot)
								if err != nil {
									logrus.Errorln(err)
								}
							}
						}
					}()
				}
			}
		case update.CallbackQuery != nil:
			updateQuery := *update.CallbackQuery
			args := strings.Split(updateQuery.Data, " ")
			if len(args) < 2 {
				continue
			}
			switch args[0] {
			case "music":
				go func() {
					err := processCallbackMusic(args, updateQuery, bot)
					if err != nil {
						logrus.Errorln(err)
					}
				}()
			}
		case update.InlineQuery != nil:
			updateQuery := *update.InlineQuery
			switch {
			case updateQuery.Query == "help":
				go func() {
					err = processInlineHelp(updateQuery, bot)
					if err != nil {
						logrus.Errorln(err)
					}
				}()
			case strings.Contains(updateQuery.Query, "search"):
				go func() {
					err = processInlineSearch(updateQuery, bot)
					if err != nil {
						logrus.Errorln(err)
					}
				}()
			default:
				go func() {
					musicID, _ := strconv.Atoi(linkTestMusic(updateQuery.Query))
					if musicID != 0 {
						err = processInlineMusic(musicID, updateQuery, bot)
						if err != nil {
							logrus.Errorln(err)
						}
					} else {
						err = processEmptyInline(updateQuery, bot)
						if err != nil {
							logrus.Errorln(err)
						}
					}
				}()
			}
		}
	}
	return 0
}
