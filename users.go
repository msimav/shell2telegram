package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/Syfaro/telegram-bot-api"
)

// User - one telegram user who interact with bot
type User struct {
	UserName       string
	FirstName      string
	LastName       string
	AuthCode       string
	AuthCodeRoot   string
	IsAuthorized   bool
	IsRoot         bool
	PrivateChatID  int
	Counter        int
	LastAccessTime time.Time
}

// Users in chat
type Users struct {
	list         map[int]*User
	allowedUsers map[string]bool
	rootUsers    map[string]bool
}

// length of random code in bytes
const CODE_BYTES_LENGTH = 15

// clear old users after 20 minutes after login
const SECONDS_FOR_OLD_USERS_BEFORE_VACUUM = 1200

// NewUsers - create Users object
func NewUsers(appConfig Config) Users {
	users := Users{
		list:         map[int]*User{},
		allowedUsers: map[string]bool{},
		rootUsers:    map[string]bool{},
	}

	for _, name := range appConfig.allowUsers {
		users.allowedUsers[name] = true
	}
	for _, name := range appConfig.rootUsers {
		users.allowedUsers[name] = true
		users.rootUsers[name] = true
	}
	return users
}

// AddNew - add new user if not exists
func (users Users) AddNew(tgbotMessage tgbotapi.Message) {
	privateChatID := 0
	if !tgbotMessage.IsGroup() {
		privateChatID = tgbotMessage.Chat.ID
	}

	if _, ok := users.list[tgbotMessage.From.ID]; ok && privateChatID > 0 {
		users.list[tgbotMessage.From.ID].PrivateChatID = privateChatID
	} else if !ok {
		users.list[tgbotMessage.From.ID] = &User{
			UserName:      tgbotMessage.From.UserName,
			FirstName:     tgbotMessage.From.FirstName,
			LastName:      tgbotMessage.From.LastName,
			IsAuthorized:  users.allowedUsers[tgbotMessage.From.UserName],
			IsRoot:        users.rootUsers[tgbotMessage.From.UserName],
			PrivateChatID: privateChatID,
		}
	}

	// collect stat
	users.list[tgbotMessage.From.ID].LastAccessTime = time.Now()
	if users.list[tgbotMessage.From.ID].IsAuthorized {
		users.list[tgbotMessage.From.ID].Counter++
	}
}

// DoLogin - generate secret code
func (users Users) DoLogin(userID int, forRoot bool) string {
	code := getRandomCode()
	if forRoot {
		users.list[userID].IsRoot = false
		users.list[userID].AuthCodeRoot = code
	} else {
		users.list[userID].IsAuthorized = false
		users.list[userID].AuthCode = code
	}
	return code
}

// SetAuthorized - set user authorized or authorized as root
func (users Users) SetAuthorized(userID int, forRoot bool) {
	users.list[userID].IsAuthorized = true
	if forRoot {
		users.list[userID].IsRoot = true
		users.list[userID].AuthCodeRoot = ""
	} else {
		users.list[userID].AuthCode = ""
	}
}

// IsValidCode - check secret code for user
func (users Users) IsValidCode(userID int, code string, forRoot bool) bool {
	var result bool
	if forRoot {
		result = code != "" && code == users.list[userID].AuthCodeRoot
	} else {
		result = code != "" && code == users.list[userID].AuthCode
	}
	return result
}

// IsAuthorized - check user is authorized
func (users Users) IsAuthorized(userID int) bool {
	isAuthorized := false
	if _, ok := users.list[userID]; ok && users.list[userID].IsAuthorized {
		isAuthorized = true
	}

	return isAuthorized
}

// IsRoot - check user is root
func (users Users) IsRoot(userID int) bool {
	isRoot := false
	if _, ok := users.list[userID]; ok && users.list[userID].IsRoot {
		isRoot = true
	}

	return isRoot
}

// broadcastForRoots - send message to all root users
func (users Users) broadcastForRoots(bot *tgbotapi.BotAPI, message string) {
	for _, user := range users.list {
		if user.IsRoot && user.PrivateChatID > 0 {
			sendMessageWithLogging(bot, user.PrivateChatID, message)
		}
	}
}

// String - format user name
func (users Users) String(userID int) string {
	result := fmt.Sprintf("%s %s", users.list[userID].FirstName, users.list[userID].LastName)
	if users.list[userID].UserName != "" {
		result += fmt.Sprintf(" (@%s)", users.list[userID].UserName)
	}
	return result
}

// clearOldUsers - clear old users without login
func (users Users) clearOldUsers() {
	for id, user := range users.list {
		if !user.IsAuthorized && !user.IsRoot && user.Counter == 0 &&
			time.Now().Sub(user.LastAccessTime).Seconds() > SECONDS_FOR_OLD_USERS_BEFORE_VACUUM {
			log.Printf("Vacuum: %d, %s", id, users.String(id))
			delete(users.list, id)
		}
	}
}

// getUserIDByName - find user by login
func (users Users) getUserIDByName(userName string) int {
	userID := 0
	for id, user := range users.list {
		if userName == user.UserName {
			userID = id
			break
		}
	}

	return userID
}

// banUser - ban user by ID
func (users Users) banUser(userID int) bool {
	if _, ok := users.list[userID]; ok {
		users.list[userID].IsAuthorized = false
		users.list[userID].IsRoot = false
		return true
	}

	return false
}

// getRandomCode - generate random code for authorize user
func getRandomCode() string {
	buffer := make([]byte, CODE_BYTES_LENGTH)
	_, err := rand.Read(buffer)
	if err != nil {
		log.Print("Get code error: ", err)
		return ""
	}

	return base64.URLEncoding.EncodeToString(buffer)
}
