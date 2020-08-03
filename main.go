package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	refreshRate           = 3                          // Refresh rate for new messages
	baseTelegramUrl       = "https://api.telegram.org" // Telegram api base url
	getUpdatesUri         = "getUpdates"               // API endpoint get updates
	sendMessageUrl        = "sendMessage"              // API endpoint send message
	telegramToken         = "Insert you token here"    // Bot auth token
	defaultHandlerMessage = "_default"                 // Default handler name in map
	defaultRoomName       = "ChackChack"               // Create room default name
	defaultPrivateStatus  = true                       // Default private status
	defaultRoomLimit      = 10                         // Default max users in one room
)

// Interface for message handler
type MainMessageHandler func(UpdateResultMessageT, *map[int]*Room, *map[int]*Room)
type RoomMessageHandler func(UpdateResultMessageT, *Room, *map[int]*Room)

/*
	Telegram api data
*/
type UpdateT struct {
	Ok     bool            `json:"ok"`
	Result []UpdateResultT `json:"result"`
}

type UpdateResultT struct {
	UpdateId int                  `json:"update_id"`
	Message  UpdateResultMessageT `json:"message"`
}

type UpdateResultMessageT struct {
	MessageId int               `json:"message_id"`
	From      UpdateResultFromT `json:"from"`
	Chat      UpdateResultChatT `json:"chat"`
	Date      int               `json:"date"`
	Text      string            `json:"text"`
}

type UpdateResultFromT struct {
	Id        int    `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Language  string `json:"language_code"`
}

type UpdateResultChatT struct {
	Id        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Type      string `json:"type"`
}

type SendMessageResponseT struct {
	Ok     bool               `json:"ok"`
	Result ResultSendMessageT `json:"result"`
}

type ResultSendMessageT struct {
	MessageID int                `json:"message_id"`
	From      FromResultMessageT `json:"from"`
}

type FromResultMessageT struct {
	Id        int    `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

/*
	Room info
*/
type Room struct {
	ID      int    // Уникальные идентификатор чата
	Name    string // Имя чата
	Limit   int    // Максимально колчество учвстников в чате
	Members []int  // Тут хранятся ID чатов с пользователями
	Private bool   // Флаг приватности. Влияет на вывод /list
}

func getUpdates(offset int) (UpdateT, error) {
	/*
		Get updates from telegram chanel
	*/
	method := getUpdatesUri
	if offset != 0 {
		method += "?offset=" + strconv.Itoa(offset)
	}
	response := sendRequest(method)
	update := UpdateT{}
	err := json.Unmarshal(response, &update)
	if err != nil {
		return update, err
	}
	return update, nil
}

func sendMessage(chatId int, text string) (SendMessageResponseT, error) {
	/*
		Send message for user with chat id
	*/
	method := sendMessageUrl + "?chat_id=" + strconv.Itoa(chatId) + "&text=" + url.QueryEscape(text)
	response := sendRequest(method)
	sendMessage := SendMessageResponseT{}

	err := json.Unmarshal(response, &sendMessage)
	if err != nil {
		return sendMessage, err
	}
	return sendMessage, nil
}

func sendRequest(method string) []byte {
	/*
		Default method send request to telegram api
	*/
	sendURL := baseTelegramUrl + "/bot" + telegramToken + "/" + method
	response := make([]byte, 0)
	resp, err := http.Get(sendURL)
	if err != nil {
		log.Println(err)
		return response
	}

	defer resp.Body.Close()

	for true {
		bs := make([]byte, 1024)
		n, err := resp.Body.Read(bs)
		response = append(response, bs[:n]...)

		if n == 0 || err != nil {
			break
		}
	}

	return response
}

func startHandler(message UpdateResultMessageT, clientsRoom *map[int]*Room, allRooms *map[int]*Room) {
	/*
		Handler on command /start.
		This is first message for user
	*/
	userMessage := "Привет " + message.From.FirstName + "!\n" +
		"Набери сообщение /help для просмотра команд"
	_, err := sendMessage(message.Chat.Id, userMessage)
	if err != nil {
		log.Println(err.Error())
	}
}

func joinRoomHandler(message UpdateResultMessageT, clientsRoom *map[int]*Room, allRooms *map[int]*Room) {
	/*
		When user join to room handler
	*/
	command := strings.Fields(message.Text)
	commandLen := len(command)
	if commandLen < 2 {
		// Check id room
		_, err := sendMessage(message.Chat.Id, "Необходимо ввести ID комнаты")
		if err != nil {
			log.Println(err.Error())
		}
		return
	}

	// Try convert room number to int
	chatID, err := strconv.Atoi(command[1])

	if err != nil {
		// Не то. так и пишим клиенту
		_, err := sendMessage(message.Chat.Id, "Неверно введен ID комнаты")
		if err != nil {
			log.Println(err.Error())
		}
		return
	}

	allRoomsInstance := *allRooms

	if roomPointer, KeyExists := allRoomsInstance[chatID]; KeyExists {
		roomInstance := *roomPointer

		if roomInstance.Limit <= len(roomInstance.Members) {
			_, err := sendMessage(message.Chat.Id, "Комната переполнена")
			if err != nil {
				log.Println(err.Error())
			}
			return
		}

		roomInstance.Members = append(roomInstance.Members, message.Chat.Id)
		*roomPointer = roomInstance

		clientsRoomInstance := *clientsRoom
		clientsRoomInstance[message.Chat.Id] = roomPointer
		*clientsRoom = clientsRoomInstance

		log.Println("Client " + strconv.Itoa(message.Chat.Id) + " entered the room " + roomInstance.Name)
		_, err := sendMessage(message.Chat.Id, "Вы успешно зашли в комнату "+roomInstance.Name)
		if err != nil {
			log.Println(err.Error())
		}
	} else {
		_, err := sendMessage(message.Chat.Id, "Комната с ID "+strconv.Itoa(chatID)+" не найдена")
		if err != nil {
			log.Println(err.Error())
		}
	}
}

func listHandler(message UpdateResultMessageT, clientsRoom *map[int]*Room, allRooms *map[int]*Room) {
	/*
		get list public rooms
	*/

	allRoomsInstance := *allRooms
	roomCount := 0
	userMessage := ""

	for _, roomPointer := range allRoomsInstance {
		roomInstance := *roomPointer
		if roomInstance.Private {
			continue
		}
		roomCount++
		userMessage += "\n" + roomInstance.Name + ", " + strconv.Itoa(roomInstance.ID) + ", " +
			strconv.Itoa(len(roomInstance.Members)) + "/" + strconv.Itoa(roomInstance.Limit)
	}
	userMessage += "\nОбщее количество публичных комнат: " + strconv.Itoa(roomCount)
	_, err := sendMessage(message.Chat.Id, userMessage)
	if err != nil {
		log.Println(err.Error())
	}
}

func helpHandler(message UpdateResultMessageT, clientsRoom *map[int]*Room, allRooms *map[int]*Room) {
	/*
		/help command handler
	*/
	helpMessage := "Вот что я умею\n" +
		"/help - Показать меню помощи\n" +
		"/join <room id> присоединиться к комнате\n" +
		"/create <room name | optional> <limit | optional > <private | optional, default True> Создать комнату.\n" +
		"/info информацию по комнате\n" +
		"/list список доступных комнат\n" +
		"/quit выйти из комнаты"
	_, err := sendMessage(message.Chat.Id, helpMessage)
	if err != nil {
		log.Println(err.Error())
	}
}

func init() {
	log.SetPrefix("privateRoomBot: ")
	log.SetFlags(log.Ldate | log.Lmicroseconds)
	log.Println("Start work bot")
}

func messageRoomHandler(message UpdateResultMessageT, room *Room, clientsRooms *map[int]*Room) {
	/*
		send messages to all room members
	*/
	roomInstance := *room
	for _, memberId := range roomInstance.Members {
		// Don't send message for author
		if memberId == message.Chat.Id {
			continue
		}
		_, err := sendMessage(memberId, message.From.FirstName+": "+message.Text)
		if err != nil {
			log.Println("error while send message: " + err.Error())
		}
	}
}

func quitRoomHandler(message UpdateResultMessageT, room *Room, clientsRooms *map[int]*Room) {
	roomValue := *room
	clientsRoomsValue := *clientsRooms

	for i, value := range roomValue.Members {
		if value == message.Chat.Id {
			roomValue.Members[i] = roomValue.Members[len(roomValue.Members)-1] // Copy last element to index i.
			roomValue.Members[len(roomValue.Members)-1] = 0                    // Erase last element (write zero value).
			roomValue.Members = roomValue.Members[:len(roomValue.Members)-1]   // Truncate slice.
		}
	}

	delete(clientsRoomsValue, message.Chat.Id)
	log.Println("delete chat " + strconv.Itoa(message.Chat.Id) + " from room " + roomValue.Name)
	log.Println(roomValue.Members)
	_, err := sendMessage(message.Chat.Id, "Вы вышли из комнаты: "+roomValue.Name)
	if err != nil {
		log.Println(err)
	}

	*clientsRooms = clientsRoomsValue
	*room = roomValue
}

func defaultMainHandler(message UpdateResultMessageT, clientsRoom *map[int]*Room, allRooms *map[int]*Room) {
	randomMessages := []string{
		"Я правда пытаюсь тебя понять, но не могу. Может всё таки /help",
		"ДА! Я ЗНАЛ, что тут есть кто-то живой!",
		"А-а-а! Ты выглядишь ужа... Ты выглядишь здорово! На самом деле, здорово.",
		"Ладно, слушай, говорю тебе прямо. Ты - последний испытуемый. И если ты мне не поможешь, мы оба умрем. Понятно? Я не хотел это говорить, но ты из меня это вытянула. Понятно? Умрем. Dos Muerte",
		"Ладно. Не хотел говорить тебе, но придется. У нас тут серьезные неприятности",
		"Пф-ф. Серьезно? И что тогда?",
		"Так, слушай. Нам надо договориться, понимаешь? Чтобы ответы были одинаковые. Если кто-нибудь спросит - не, никто не спросит, ты не волнуйся - но если кто-нибудь спросит, скажи, что последний раз, когда ты проверяла, все были более-менее живы. Хорошо? Не мертвы",
		"Ох, уже близко... Тебе видно? Я пролезу? Места хватит?",
		"Подожди. Это немножко сложно.",
		"Ох, просто открой дверь! Это было слишком агрессивно... Привет, друг! Открой, пожалуйста, дверь!",
		"Но не тревожься, потому что... Хотя, на самом деле, если ты беспокоишься, то это нормально. Потому что беспокойство - это нормальная реакция на то, что у тебя поврежден мозг. Так что, если ты чувствуешь беспокойство, это может означать, что мозг поврежден не так сильно. Хотя, скорее всего, сильно.",
		"Мы почти на месте! Помни, что тебе нужна такая пушка, которая делает отверстия. Не пулевые отверстия, а... Ну, ты разберешься. Давай, соберись с духом!",
	}
	_, err := sendMessage(message.Chat.Id, randomMessages[rand.Intn(len(randomMessages))])
	if err != nil {
		log.Println(err.Error())
	}
}

func cleanRooms(rooms *map[int]*Room) {
	roomsMapInstance := *rooms
	for roomID, roomPointer := range roomsMapInstance {
		roomInstance := *roomPointer
		roomMembersLen := len(roomInstance.Members)
		if roomMembersLen == 0 {
			log.Println("Room " + roomInstance.Name + "is empty. delete it.")
			delete(roomsMapInstance, roomID)
		}
	}
	*rooms = roomsMapInstance
}

func createRoomHandler(message UpdateResultMessageT, userRooms *map[int]*Room, allrooms *map[int]*Room) {
	allID := make([]int, 10)
	allRoomsInstance := *allrooms
	userRoomsInstance := *userRooms
	for key, _ := range allRoomsInstance {
		allID = append(allID, key)
	}

	nextID := 0

	if len(allID) != 0 {
		sort.Ints(allID)
		nextID = allID[len(allID)-1] + 1
	}

	command := strings.Fields(message.Text)
	roomName := defaultRoomName

	if len(command) >= 2 {
		roomName = command[1]
	}

	roomLimit := defaultRoomLimit
	if len(command) >= 3 {
		var err error
		roomLimit, err = strconv.Atoi(command[2])
		if err != nil {
			_, e := sendMessage(message.Chat.Id, "Не правильно введен лимит для комнаты")
			if e != nil {
				log.Println(e.Error())
			}
		}
	}

	roomPrivate := defaultPrivateStatus
	if len(command) >= 3 {
		roomPrivate = !roomPrivate
	}

	newRoom := Room{
		ID:      nextID,
		Name:    roomName,
		Limit:   roomLimit,
		Private: roomPrivate,
		Members: []int{message.Chat.Id},
	}

	allRoomsInstance[nextID] = &newRoom
	userRoomsInstance[message.Chat.Id] = allRoomsInstance[nextID]
	log.Println("create room: " + strconv.Itoa(newRoom.ID) + newRoom.Name)

	*allrooms = allRoomsInstance
	*userRooms = userRoomsInstance

	// Send message for user
	_, err := sendMessage(message.Chat.Id,
		"Вы создали комнату: "+newRoom.Name+"\nID комнаты: "+strconv.Itoa(newRoom.ID)+
			"\nЛимит комнаты: "+strconv.Itoa(newRoom.Limit))

	if err != nil {
		log.Println(err.Error())
	}
}

func roomInfoHandler(message UpdateResultMessageT, room *Room, clientsRooms *map[int]*Room) {
	roomInstance := *room
	roomLimit := strconv.Itoa(roomInstance.Limit)
	roomID := strconv.Itoa(roomInstance.ID)
	membersLen := strconv.Itoa(len(roomInstance.Members))
	roomName := roomInstance.Name

	_, err := sendMessage(message.Chat.Id,
		"Вы сейчас находитесь в комнате: "+roomName+", id: "+roomID+"\n"+
			"Количкство участников: "+membersLen+"\n"+
			"Лимит участников: "+roomLimit)
	if err != nil {
		log.Println(err.Error())
	}
}

func main() {
	mainDispatcher := map[string]MainMessageHandler{
		"/start":              startHandler,
		"/help":               helpHandler,
		"/create":             createRoomHandler,
		"/join":               joinRoomHandler,
		"/list":               listHandler,
		defaultHandlerMessage: defaultMainHandler,
	}

	roomDispatcher := map[string]RoomMessageHandler{
		defaultHandlerMessage: messageRoomHandler,
		"/quit":               quitRoomHandler,
		"/info":               roomInfoHandler,
	}

	var clientsRooms map[int]*Room = make(map[int]*Room)
	var allRooms map[int]*Room = make(map[int]*Room)

	offset := 0

	for {
		time.Sleep(1 * refreshRate)
		cleanRooms(&allRooms)

		update, err := getUpdates(offset)
		if err != nil {
			log.Println("error while receiving updates: " + err.Error())
			continue
		}

		updatesLen := len(update.Result)
		if updatesLen == 0 {
			continue
		}

		log.Println("received " + strconv.Itoa(updatesLen) + " messages")

		for _, item := range update.Result {
			if item.Message.Text == "" {
				continue
			}

			command := strings.Fields(item.Message.Text)[0]

			if room, keyExists := clientsRooms[item.Message.Chat.Id]; keyExists {
				if value, keyExists := roomDispatcher[command]; keyExists {
					value(item.Message, room, &clientsRooms)
				} else {
					roomDispatcher[defaultHandlerMessage](item.Message, room, &clientsRooms)
				}
			} else if value, keyExists := mainDispatcher[command]; keyExists {
				value(item.Message, &clientsRooms, &allRooms)
			} else {
				mainDispatcher[defaultHandlerMessage](item.Message, &clientsRooms, &allRooms)
			}
		}
		offset = update.Result[updatesLen-1].UpdateId + 1
	}
}
