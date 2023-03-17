package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NicoNex/echotron/v3"
)

type stateFn func(*echotron.Update) stateFn

type TelegramBot struct {
	storage   Storage
	douWorker *DouWorker
}

type bot struct {
	telegramBot *TelegramBot
	chatID      int64
	category    DouCategory
	state       stateFn
	messagesIds []int
	lock        *sync.RWMutex
	spamData    []int64
	echotron.API
}

var token = os.Getenv("TG")

var dsp *echotron.Dispatcher
var parseModeHTML = &echotron.MessageOptions{ParseMode: echotron.HTML}

func CreateTelegramBot(storage Storage, douWorker *DouWorker) *TelegramBot {
	telegramBot := &TelegramBot{
		storage:   storage,
		douWorker: douWorker,
	}
	return telegramBot
}

func (tb *TelegramBot) Run() {
	go pullVacancies(tb)
	dsp = echotron.NewDispatcher(token, func(chatID int64) echotron.Bot {
		bot := newBot(chatID).(*bot)
		bot.telegramBot = tb
		return bot
	})
	log.Println(dsp.Poll())
}

func newBot(chatID int64) echotron.Bot {
	bot := &bot{
		chatID:   chatID,
		API:      echotron.NewAPI(token),
		lock:     &sync.RWMutex{},
		spamData: make([]int64, 3),
	}
	bot.state = bot.handleMessage
	go bot.selfDestruct(time.After(10 * time.Minute))
	return bot
}

func newBotBroadcast(chatID int64) echotron.Bot {
	bot := &bot{
		chatID: chatID,
		API:    echotron.NewAPI(token),
	}
	bot.state = bot.handleMessage
	go bot.selfDestruct(time.After(10 * time.Second))
	return bot
}

func (b *bot) CheckForSpam(msgTime int64) bool {
	b.spamData[0] = msgTime

	var timeBetweenMessages int64 = 0
	for i := 0; i < len(b.spamData)-1; i++ {
		timeBetweenMessages += b.spamData[i] - b.spamData[i+1]
	}

	for i := len(b.spamData) - 2; i >= 0; i-- {
		b.spamData[i+1] = b.spamData[i]
		b.spamData[i] = 0
	}

	return int(timeBetweenMessages) < len(b.spamData)*700
}

func (b *bot) selfDestruct(timech <-chan time.Time) {
	<-timech
	if b.lock != nil {
		b.lock.Lock()
		defer b.lock.Unlock()
	}

	b.RemoveMessages()
	dsp.DelSession(b.chatID)
}

func (b *bot) Update(update *echotron.Update) {
	if update == nil || update.Message == nil {
		fmt.Println("destroy session sync issue")
		return
	}

	msgTime := time.Now().UnixMilli()
	b.lock.Lock()
	defer b.lock.Unlock()

	b.SendChatAction(echotron.Typing, b.chatID, nil)
	if spam := b.CheckForSpam(msgTime); spam {
		b.SendAutoDeleteMessage("не спамь будь ласка😉", b.chatID, parseModeHTML)
		b.AddLastMessageToDeleteList(update)
		b.state = b.handleMessage
		return
	}

	b.state = b.state(update)
	b.AddLastMessageToDeleteList(update)
}

func (b *bot) AddLastMessageToDeleteList(update *echotron.Update) {
	b.messagesIds = append(b.messagesIds, update.Message.ID)
}

func (b *bot) RemoveMessages() {
	toKeep := []int{}
	for i := 0; i < len(b.messagesIds); i++ {
		res, err := b.DeleteMessage(b.chatID, b.messagesIds[i])
		if err != nil && res.ErrorCode != http.StatusBadRequest {
			toKeep = append(toKeep, b.messagesIds[i])
		}
	}
	b.messagesIds = toKeep
}

func (b *bot) handleMessage(update *echotron.Update) stateFn {
	r := b.handleCommands(update)
	if r != nil {
		return r
	}
	msg := "👇<b>Список команд</b>👇\n\n"
	msg += "<i>/follow</i> Підписатися на розсилку, та отримувати нові вакансії за категоріями, які ви самі оберете\n\n"
	msg += "<i>/unfollow</i> Відписатися від розсилки за категоріями\n\n"
	msg += "<i>/myfollows</i> Ваші поточні підписки"
	b.SendAutoDeleteMessage(msg, b.chatID, parseModeHTML)

	return b.handleMessage
}
func (b *bot) handleCommands(update *echotron.Update) stateFn {
	if update.Message.Text == "/follow" {
		return b.handleSubscribe(update)
	}
	if update.Message.Text == "/unfollow" {
		return b.handleUnsubscribe(update)
	}
	if update.Message.Text == "/myfollows" {
		return b.handleMySubcriptions(update)
	}

	return nil
}

func (b *bot) handleMySubcriptions(update *echotron.Update) stateFn {
	subInfo, state := b.getCurrentSubscriptionStatus(update)
	if subInfo == nil {
		return state
	}

	subs := []string{}
	for _, subCat := range subInfo.Subscriptions {
		for v, filter := range b.telegramBot.douWorker.experienceFilters {
			if DBIdToId(subCat.Experience) == filter {
				s := fmt.Sprintf("%s(%s)", subCat.NameCategory, v)
				subs = append(subs, s)
			}
		}
	}

	b.SendAutoDeleteMessage(fmt.Sprintf("✅ Ви підписані на: <b>%s</b>", strings.Join(subs, ", ")), b.chatID, parseModeHTML)
	return b.handleMessage
}

func (b *bot) handleSubscribe(update *echotron.Update) stateFn {
	btns := [][]echotron.KeyboardButton{}
	for id, category := range b.telegramBot.douWorker.categories {
		if id%3 == 0 {
			btns = append(btns, []echotron.KeyboardButton{})
		}
		btns[len(btns)-1] = append(btns[len(btns)-1], echotron.KeyboardButton{Text: category.name})
	}
	options := echotron.MessageOptions{
		ReplyMarkup: echotron.ReplyKeyboardMarkup{
			Keyboard:        btns,
			OneTimeKeyboard: true,
		},
	}

	b.SendAutoDeleteMessage("🎯 Оберіть категорію, за якою ви бажаете отримувати повідомлення про нові вакансії, щойно вони з'являються на DOU", b.chatID, &options)

	return b.handleSubscribeForCategory
}

func (b *bot) handleCategoryExperience(update *echotron.Update) stateFn {
	r := b.handleCommands(update)
	if r != nil {
		return r
	}

	exp, err := b.findExperience(update.Message.Text)
	if err != nil {
		b.SendAutoDeleteMessage("🚫 Ви обрали не існуючий досвід", b.chatID, parseModeHTML)
		return b.handleMessage
	}

	ok, err := b.telegramBot.storage.SubscribeUser(b.category, exp, int(update.Message.From.ID), b.chatID, update.Message.From.Username)
	if err != nil {
		fmt.Println(err)
		b.SendAutoDeleteMessage("🚫 Не вдалося підписатися, спробуйте ще", b.chatID, parseModeHTML)
		return b.handleMessage
	}

	if !ok {
		b.SendAutoDeleteMessage(fmt.Sprintf("‼️ Ви вже підписані на <b>%s</b>", b.category.name), b.chatID, parseModeHTML)
		return b.handleMessage
	}

	b.SendAutoDeleteMessage(fmt.Sprintf("✅ Ви вдало підписалися на <b>%s(%s)</b>, щойно з'явиться нова вакансія - я одразу вас сповіщу👍", b.category.name, update.Message.Text),
		b.chatID, parseModeHTML)
	return b.handleMessage
}

func (b *bot) handleSubscribeForCategory(update *echotron.Update) stateFn {
	r := b.handleCommands(update)
	if r != nil {
		return r
	}

	category, err := b.findCategory(update.Message.Text)
	if err != nil {
		b.SendAutoDeleteMessage("🚫 Ви обрали не існуючу категорію", b.chatID, parseModeHTML)
		return b.handleMessage
	}

	b.category = category

	btns := [][]echotron.KeyboardButton{}
	i := 0
	for k, _ := range b.telegramBot.douWorker.experienceFilters {
		if i%3 == 0 {
			btns = append(btns, []echotron.KeyboardButton{})
		}
		i++
		btns[len(btns)-1] = append(btns[len(btns)-1], echotron.KeyboardButton{Text: k})
	}

	options := echotron.MessageOptions{
		ReplyMarkup: echotron.ReplyKeyboardMarkup{
			Keyboard:        btns,
			OneTimeKeyboard: true,
		}}
	b.SendAutoDeleteMessage("📈 Оберіть досвід", b.chatID, &options)

	return b.handleCategoryExperience
}

func (b *bot) getCurrentSubscriptionStatus(update *echotron.Update) (*SubscriptionInfo, stateFn) {
	subInfo, err := b.telegramBot.storage.GetSubscriptionInfo(int(update.Message.From.ID))
	if err != nil {
		fmt.Println(err)
		b.SendAutoDeleteMessage("🚫 Не вдалося отримати ваші підписки, спробуйте ще", b.chatID, parseModeHTML)
		return nil, b.handleMessage
	}

	if len(subInfo.Subscriptions) == 0 {
		b.SendAutoDeleteMessage("🚫 Ви не підписані на жодну з категорій, скористайтеся командою <b>/follow</b>", b.chatID, parseModeHTML)
		return nil, b.handleMessage
	}

	return &subInfo, nil
}

func (b *bot) handleUnsubscribe(update *echotron.Update) stateFn {
	subInfo, state := b.getCurrentSubscriptionStatus(update)
	if subInfo == nil {
		return state
	}

	btns := [][]echotron.KeyboardButton{}
	for id, category := range subInfo.Subscriptions {
		if id%3 == 0 {
			btns = append(btns, []echotron.KeyboardButton{})
		}
		btns[len(btns)-1] = append(btns[len(btns)-1], echotron.KeyboardButton{Text: category.NameCategory})
	}

	options := echotron.MessageOptions{
		ReplyMarkup: echotron.ReplyKeyboardMarkup{
			Keyboard:        btns,
			OneTimeKeyboard: true,
		}}
	b.SendAutoDeleteMessage("👁 Оберіть категорію для відписки", b.chatID, &options)
	return b.handleUnsubscribeFromCategory
}

func (b *bot) handleUnsubscribeFromCategory(update *echotron.Update) stateFn {
	r := b.handleCommands(update)
	if r != nil {
		return r
	}

	ok, err := b.telegramBot.storage.UnsubscribeUser(update.Message.Text, int(update.Message.From.ID))
	if err != nil {
		fmt.Println(err)
		b.SendAutoDeleteMessage("🚫 Не вдалося видалитии підписку, спробуйте ще", b.chatID, parseModeHTML)
		return b.handleMessage
	}

	if !ok {
		b.SendAutoDeleteMessage("🚫 У вас немае підписки на: "+update.Message.Text, b.chatID, parseModeHTML)
		return b.handleMessage
	}

	b.SendAutoDeleteMessage(fmt.Sprintf("✅ Підписка на <b>%s</b> видаленна ", update.Message.Text), b.chatID, parseModeHTML)
	return b.handleMessage
}

func (b *bot) findCategory(name string) (DouCategory, error) {
	for _, c := range b.telegramBot.douWorker.categories {
		if c.name == name {
			return c, nil
		}
	}
	return DouCategory{}, fmt.Errorf("Category `%s` wasn't found", name)
}

func (b *bot) findExperience(name string) (string, error) {
	for k, v := range b.telegramBot.douWorker.experienceFilters {
		if k == name {
			return v, nil
		}
	}
	return "", fmt.Errorf("Experience `%s` wasn't found", name)

}

func (b *bot) SendAutoDeleteMessage(text string, chatID int64, opts *echotron.MessageOptions) {
	b.RemoveMessages()
	res, err := b.SendMessage(text, chatID, opts)
	if err != nil {
		fmt.Println(err)
		return
	}
	b.messagesIds = append(b.messagesIds, res.Result.ID)
}

func pullVacancies(tb *TelegramBot) {
	for {
		vacancy := <-tb.douWorker.newVacancyChan
		subs, err := tb.storage.GetAllSubscribers(vacancy.categoryName, vacancy.categoryId, vacancy.experience)
		if err != nil {
			fmt.Println(err)
			continue
		}

		for _, sub := range subs {
			fmt.Printf("Sending Vacancy to subscriber(%s): %+v\n", sub.UserName, vacancy)
			b := newBotBroadcast(sub.ChatId).(*bot)
			msg := fmt.Sprintf("🔥<b>Нова вакансія🔥</b>\n\n <b>Категорія</b>: <i>%s</i> 👀 \n\n➡️%s\n%s", vacancy.categoryName, vacancy.name, vacancy.url)
			b.SendMessage(msg, sub.ChatId, parseModeHTML)
			time.Sleep(100 * time.Millisecond)
		}
	}
}
