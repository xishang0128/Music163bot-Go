package bot

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/XiaoMengXinX/Music163Api-Go/api"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func processSearch(message tgbotapi.Message, bot *tgbotapi.BotAPI) (err error) {
	var msgResult tgbotapi.Message
	if message.CommandArguments() == "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, inputKeyword)
		msg.ReplyToMessageID = message.MessageID
		msgResult, err = bot.Send(msg)
		return err
	}
	msg := tgbotapi.NewMessage(message.Chat.ID, searching)
	msg.ReplyToMessageID = message.MessageID
	msgResult, err = bot.Send(msg)
	if err != nil {
		return err
	}
	searchResult, _ := api.SearchSong(data, api.SearchSongConfig{
		Keyword: message.CommandArguments(),
		Limit:   10,
	})
	if len(searchResult.Result.Songs) == 0 {
		newEditMsg := tgbotapi.NewEditMessageText(message.Chat.ID, msgResult.MessageID, noResults)
		msgResult, err = bot.Send(newEditMsg)
		return err
	}
	var inlineButton []tgbotapi.InlineKeyboardButton
	var textMessage string
	for i := 0; i < len(searchResult.Result.Songs) && i < 8; i++ {
		var songArtists string
		for i, artist := range searchResult.Result.Songs[i].Artists {
			if i == 0 {
				songArtists = artist.Name
			} else {
				songArtists = fmt.Sprintf("%s/%s", songArtists, artist.Name)
			}
		}
		inlineButton = append(inlineButton, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d", i+1), fmt.Sprintf("music %d %d", searchResult.Result.Songs[i].Id, message.MessageID)))
		textMessage = fmt.Sprintf("%s%d.「%s」 - %s\n", textMessage, i+1, searchResult.Result.Songs[i].Name, songArtists)
	}
	var numericKeyboard = tgbotapi.NewInlineKeyboardMarkup(inlineButton)
	newEditMsg := tgbotapi.NewEditMessageText(message.Chat.ID, msgResult.MessageID, textMessage)
	newEditMsg.ReplyMarkup = &numericKeyboard
	_, err = bot.Send(newEditMsg)
	return err
}

func handleCallbackQuery(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	callbackQuery := update.CallbackQuery
	if callbackQuery == nil {
		return nil
	}

	data := callbackQuery.Data

	if strings.HasPrefix(data, "music ") {
		args := strings.Split(data, " ")
		musicID, _ := strconv.Atoi(string(args[1]))
		replyID, _ := strconv.Atoi(string(args[2]))

		deleteMsg := tgbotapi.NewDeleteMessage(callbackQuery.Message.Chat.ID, callbackQuery.Message.MessageID)
		bot.Send(deleteMsg)

		err := processMusic(musicID, replyID, *callbackQuery.Message, bot)
		if err != nil {
			return err
		}
	}
	return nil
}
