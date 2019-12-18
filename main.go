package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	strip "github.com/grokify/html-strip-tags-go"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const FinalResults = "شاخص امروز: %s\n%s\n\nشاخص دیروز: %s\n%s"
const Version = "1.0.0/Build 1"

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please pass your bot token as argument.")
	}
	bot, err := tgbotapi.NewBotAPI(os.Args[1])
	if err != nil {
		panic("Cannot initialize the bot: " + err.Error())
	}
	log.Printf("Bot authorized on account %s", bot.Self.UserName)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, _ := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil && update.InlineQuery == nil {
			continue
		}
		if update.InlineQuery != nil {
			inline := tgbotapi.InlineConfig{
				InlineQueryID: update.InlineQuery.ID,
				IsPersonal:    true,
				CacheTime:     0,
				Results:       nil,
			}
			now, nowInt, yesterday, nowDetails, yesterdayDetails, err := GetStatus()
			if err != nil {
				inline.Results = []interface{}{tgbotapi.NewInlineQueryResultArticle(update.InlineQuery.ID, "Error", "Cannot get results :(")}
			} else {
				toSend := tgbotapi.NewInlineQueryResultArticle(update.InlineQuery.ID, now, fmt.Sprintf(FinalResults, now, nowDetails, yesterday, yesterdayDetails))
				toSend.ThumbURL = "https://github.com/HirbodBehnam/TehranAirNowBot/raw/master/" + strconv.FormatInt(int64(nowInt), 10) + ".jpg"
				toSend.Description = fmt.Sprintf(FinalResults, now, nowDetails, yesterday, yesterdayDetails)
				inline.Results = []interface{}{toSend}
			}
			_, _ = bot.AnswerInlineQuery(inline)
			continue
		}
		if update.Message.IsCommand() {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
			switch update.Message.Command() {
			case "start":
				msg.Text = "یک بات ساده برای گرفتن میزان آلودگی هوای تهران از سایت https://airnow.tehran.ir/. برای گرفتن میزان آلودگی هوا از /airnow استفاده کنید."
			case "help":
				msg.Text = "برای گرفتن وضعیت هوا از /airnow استفاده کنید.\nشما می توانید این ربات را به گروه های خود اد کنید یا اینکه از Inline Mode برای گرفتن اطلاعات نیز استفاده کنید.\nتمامی دیتا ها از سایت https://airnow.tehran.ir/ جمع آوری می شوند."
				msg.DisableWebPagePreview = true
			case "about":
				msg.Text = "A simple bot by Hirbod Behnam\nSource at https://github.com/HirbodBehnam/TehranAirNowBot\nVersion " + Version
			case "airnow":
				go func(fUpdate tgbotapi.Update) {
					// Request the HTML page.
					message := tgbotapi.NewMessage(fUpdate.Message.Chat.ID, "چند لحظه صبر کنید...")
					message.ReplyToMessageID = fUpdate.Message.MessageID
					sentMessage, err := bot.Send(message)
					if err != nil {
						log.Println("Cannot send message:", err.Error())
						return
					}

					now, intNow, yesterday, nowDetails, yesterdayDetails, err := GetStatus()
					if err != nil {
						_, _ = bot.Send(tgbotapi.NewEditMessageText(fUpdate.Message.Chat.ID, sentMessage.MessageID, "Error on getting results: "+err.Error()))
						return
					}

					if intNow < 0 {
						messagePic := tgbotapi.NewMessage(fUpdate.Message.Chat.ID, fmt.Sprintf(FinalResults, now, nowDetails, yesterday, yesterdayDetails))
						messagePic.ReplyToMessageID = fUpdate.Message.MessageID
						_, _ = bot.Send(messagePic)
					} else { //Check cache This means empty cache; upload the photo
						messagePic := tgbotapi.NewPhotoShare(fUpdate.Message.Chat.ID, "https://github.com/HirbodBehnam/TehranAirNowBot/raw/master/"+strconv.FormatInt(int64(intNow), 10)+".jpg")
						messagePic.Caption = fmt.Sprintf(FinalResults, now, nowDetails, yesterday, yesterdayDetails)
						messagePic.ReplyToMessageID = fUpdate.Message.MessageID
						_, _ = bot.Send(messagePic)
					}

					_, _ = bot.Send(tgbotapi.NewDeleteMessage(fUpdate.Message.Chat.ID, sentMessage.MessageID)) //Delete waiting message
				}(update)
				continue
			default:
				msg.Text = "I don't know that command"
			}
			_, _ = bot.Send(msg)
			continue
		}
	}
}

func GetStatus() (string, int, string, string, string, error) {
	res, err := http.Get("https://airnow.tehran.ir/")
	if err != nil {
		return "", -1, "", "", "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", -1, "", "", "", err
	}
	// Parse the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", -1, "", "", "", err
	}
	var now, yesterday, nowDetails, yesterdayDetails string
	var intNow int
	// We are searching for ID ContentPlaceHolder1_lblAqi3h and ContentPlaceHolder1_lblAqi24h
	doc.Find("#ContentPlaceHolder1_lblAqi3h").Each(func(i int, s *goquery.Selection) {
		now, _ = s.Html()
		intNow, err = strconv.Atoi(now)
		if err != nil {
			intNow = -1
		}
	})
	doc.Find("#ContentPlaceHolder1_lblAqi24h").Each(func(i int, s *goquery.Selection) {
		yesterday, _ = s.Html()
	})
	doc.Find("#ContentPlaceHolder1_lblAqi3hDesc").Each(func(i int, s *goquery.Selection) {
		nowDetails, _ = s.Html()
		nowDetails = strings.ReplaceAll(nowDetails, "<br/>", "\n")
		nowDetails = strip.StripTags(nowDetails)
	})
	doc.Find("#ContentPlaceHolder1_lblAqi24hDesc").Each(func(i int, s *goquery.Selection) {
		yesterdayDetails, _ = s.Html()
		yesterdayDetails = strings.ReplaceAll(yesterdayDetails, "<br/>", "\n")
		yesterdayDetails = strip.StripTags(yesterdayDetails)
	})
	switch {
	case intNow < 0: //Error
	case intNow <= 50:
		intNow = 0
	case intNow <= 100:
		intNow = 1
	case intNow <= 150:
		intNow = 2
	case intNow <= 200:
		intNow = 3
	case intNow <= 300:
		intNow = 4
	default:
		intNow = 5
	}
	return now, intNow, yesterday, nowDetails, yesterdayDetails, nil
}
