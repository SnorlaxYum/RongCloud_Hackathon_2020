package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type userDB struct {
	UserID      string
	Nickname    string
	Password    string
	PortraitURI string
	Token       string
	isAdmin     bool
	code        int
}

func (user *userDB) userLogin(w http.ResponseWriter, r *http.Request) error {
	userInDB := userDB{}
	userInDB.UserID = user.UserID
	err := userInDB.queryUserDB()
	if err != nil {
		panic(err)
	} else if userInDB.Password == "" {
		return errors.New("no corresponding user found")
	} else if bcrypt.CompareHashAndPassword([]byte(userInDB.Password), []byte(user.Password)) == nil {
		err = userInDB.forkAreaVerification()
		checkErr(err)
		err = createSessionTable()
		checkErr(err)
		session, err := userInDB.addToSessionTable(r)
		if session != "" {
			http.SetCookie(w, &http.Cookie{Name: "SESSIONID", Value: session, Expires: time.Now().Add(24 * time.Hour), Path: "/"})
		}
		return err
	}
	return errors.New("password not matched")
}

func (user *userDB) addNewUser() error {
	err := user.queryUserDB()
	checkErr(err)

	if len(user.Token) != 0 {
		return fmt.Errorf(`userID %s already in use`, user.UserID)
	}

	err = user.registerAPI()
	if err != nil {
		return err
	}
	newUser, err := db.Prepare(`INSERT INTO accounts(userID, nickname, portraitURI, password, created, token, isAdmin) VALUES($1, $2, $3, $4, $5, $6, $7);`)
	checkErr(err)
	password, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	checkErr(err)
	_, err = newUser.Exec(user.UserID, user.Nickname, user.PortraitURI, password, time.Now(), user.Token, false)
	user.isAdmin = false
	return err
}

func (user *userDB) addToSessionTable(r *http.Request) (string, error) {
	chkfmt, err := db.Prepare(`SELECT sessionID, userinDB, remote FROM sessions WHERE userinDB=$1;`)
	if err != nil {
		return "", err
	}
	check, err := chkfmt.Query(user.UserID)
	var currentUserSessions []userSession

	// no more than 3 sessions from a same user

	for check.Next() {
		var currentUser userSession
		check.Scan(&currentUser.sessionID, &currentUser.userinDB, &currentUser.remote)
		currentUserSessions = append(currentUserSessions, currentUser)
	}
	for len(currentUserSessions) >= 3 {
		delfmt, err := db.Prepare(`DELETE FROM sessions WHERE sessionID=$1;`)
		if err != nil {
			return "", err
		}
		_, err = delfmt.Exec(currentUserSessions[0].sessionID)
		if err != nil {
			return "", err
		}
		currentUserSessions = deleteFromArray(currentUserSessions, 0)
	}

	newS, err := db.Prepare(`INSERT INTO sessions (sessionID, userinDB, expiration, remote) VALUES ($1, $2, $3, $4);`)
	if err != nil {
		return "", err
	}
	var remote string
	if theRem := r.Header.Get("X-FORWARDED-FOR"); theRem != "" {
		remoteArray := strings.Split(theRem, ", ")
		remote = remoteArray[len(remoteArray)-1]
	} else {
		remote = strings.Split(r.RemoteAddr, ":")[0]
	}
	userID, expiration := user.UserID, time.Now().Add(24*time.Hour)
	session := uuid.New().String()
	_, err = newS.Exec(session, userID, expiration, remote)
	return session, err
}

func (user *userDB) queryUserDB() (err error) {
	userQuery := db.QueryRow(`SELECT userid, nickname, password, portraituri, token, isAdmin FROM accounts WHERE userid=$1;`, user.UserID)
	userQuery.Scan(&user.UserID, &user.Nickname, &user.Password, &user.PortraitURI, &user.Token, &user.isAdmin)
	return
}

func (user *userDB) forkAreaVerification() (err error) {
	var forkArea string
	if strings.Contains(rongURI, "cn") {
		forkArea = "cn"
	} else {
		forkArea = "sg"
	}
	if !strings.Contains(user.Token, forkArea) {
		err = user.registerAPI()
		checkErr(err)
		err = user.write()
		checkErr(err)
	}
	return
}

func (user *userDB) write() error {
	userQu := userDB{}
	userQu.UserID = user.UserID
	err := userQu.queryUserDB()
	checkErr(err)
	if userQu.Password != "" {
		// ONLY MADE FOR TOKEN CHANGE NOW
		if user.Token != "" {
			_, err := db.Exec(`UPDATE accounts SET token=$1 WHERE userID=$2;`, user.Token, user.UserID)
			checkErr(err)
		}
	}
	return err
}

func (user *userDB) registerAPI() (err error) {
	data := &url.Values{}
	data.Set("userId", user.UserID)
	data.Set("name", user.Nickname)
	data.Set("portraitURI", user.PortraitURI)

	err = user.requestRongAPI("POST", "/user/getToken.json", data)
	return
	// user, err := rongIns.UserRegister(user.UserID, user.Nickname, user.PortraitURI)
	// checkErr(err)

	// return userRes{user.Status, user.UserID, user.Token}
}

func (user *userDB) changeInfoAPI() (err error) {
	data := &url.Values{}
	data.Set("userId", user.UserID)
	data.Set("name", user.Nickname)
	data.Set("portraitURI", user.PortraitURI)

	err = user.requestRongAPI("POST", "/user/refresh.json", data)
	return
}

func (user *userDB) requestRongAPI(reqType string, uri string, data *url.Values) error {
	client := &http.Client{}
	nonce, timestamp, sig := signatureUniqueRong()

	req, _ := http.NewRequest(reqType, rongURI+uri, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("App-Key", appKey)
	req.Header.Set("Nonce", nonce)
	req.Header.Set("Timestamp", timestamp)
	req.Header.Set("Signature", sig)
	// if uri == "/user/getToken.json" {
	log.Output(1, "[Request Header] Content-Type: "+req.Header.Get("Content-Type"))
	log.Output(1, "[Request Header] App-Key: "+req.Header.Get("App-Key"))
	log.Output(1, "[Request Header] Nonce: "+req.Header.Get("Nonce"))
	log.Output(1, "[Request Header] Timestamp: "+req.Header.Get("Timestamp"))
	log.Output(1, "[Request Header] Signature: "+req.Header.Get("Signature"))

	log.Output(1, "[Request] userId: "+data.Get("userId"))
	log.Output(1, "[Request] name: "+data.Get("name"))
	log.Output(1, "[Request] portraitURI: "+data.Get("portraitURI"))
	// }

	resp, err := client.Do(req)
	checkErr(err)

	body, err := ioutil.ReadAll(resp.Body)
	checkErr(err)

	err = json.Unmarshal(body, user)

	// if uri == "/user/getToken.json" {
	log.Output(1, fmt.Sprintf("[Responce] code: %d", user.code))
	log.Output(1, "[Responce] userId: "+user.UserID)
	log.Output(1, "[Responce] token: "+user.Token)
	log.Output(1, "[Responce] from: "+rongURI+uri)
	// }

	// fmt.Println("code: ", result.Code, "userID: ", result.UserID, "token: ", result.Token)
	if user.code != 200 && user.code != 0 {
		return fmt.Errorf(`Registration failure, code %d`, user.code)
	}

	return err
}

type listedUser struct {
	UserID      string
	Nickname    string
	PortraitURI string
	IsAdmin     bool
}

type userSession struct {
	sessionID string
	userinDB  string
	remote    string
}

func (session *userSession) clear() (err error) {
	_, err = db.Exec(`DELETE FROM sessions WHERE sessionID=$1;`, session.sessionID)
	checkErr(err)

	return
}
func (session *userSession) getSessionFromRequest(r *http.Request) (err error) {
	var remote string
	if xFor := r.Header.Get("X-FORWARDED-FOR"); xFor != "" {
		remoteArray := strings.Split(xFor, ", ")
		remote = remoteArray[len(remoteArray)-1]
	} else {
		remote = strings.Split(r.RemoteAddr, ":")[0]
	}

	sessionID, err := r.Cookie("SESSIONID")
	if err != nil && strings.Contains(err.Error(), "not present") {
		err = fmt.Errorf(`Session ID not existed`)
		return
	}

	sessionQuery := db.QueryRow(`SELECT sessionid, userinDB, remote FROM sessions WHERE sessionid=$1;`, sessionID.Value)
	sessionQuery.Scan(&session.sessionID, &session.userinDB, &session.remote)

	if session.remote != remote {
		err = fmt.Errorf(`Session remote not matched`)
	}

	return
}

// userRelation
// subjectID: the user ID of subjective
// objectID: the user ID of objective
// relation: the relation between the two users (-1 - blacklisted; 1 - friend)

type userRelation struct {
	id        int
	SubjectID string
	ObjectID  string
	Relation  int
}

func (relation *userRelation) write() (err error) {
	relationExisted := userRelation{}
	relationExisted.ObjectID, relationExisted.SubjectID = relation.ObjectID, relation.SubjectID
	err = relationExisted.query()
	if err != nil {
		return fmt.Errorf(`Database Query Error: %s`, err.Error())
	}
	if relation.id == relationExisted.id {
		writePre, err := db.Prepare(`INSERT INTO userRelation (subjectID, objectID, relation) VALUES ($1, $2, $3);`)
		if err != nil {
			return fmt.Errorf(`Prepare Error: %s`, err.Error())
		}
		_, err = writePre.Exec(relation.SubjectID, relation.ObjectID, relation.Relation)
		if err != nil {
			return fmt.Errorf(`Execute Error: %s`, err.Error())
		}
	} else {
		writePre, err := db.Prepare(`UPDATE userRelation SET subjectID=$1, objectID=$2, relation=$3 WHERE id=$4;`)
		if err != nil {
			return fmt.Errorf(`Prepare Error: %s`, err.Error())
		}
		_, err = writePre.Exec(relation.SubjectID, relation.ObjectID, relation.Relation, relationExisted.id)
		if err != nil {
			return fmt.Errorf(`Execute Error: %s`, err.Error())
		}
	}
	return
}

func (relation *userRelation) query() (err error) {
	relationQuery := db.QueryRow(`SELECT id, relation FROM userRelation WHERE subjectID = $1 AND objectID = $2;`, relation.SubjectID, relation.ObjectID)
	relationQuery.Scan(&relation.id, &relation.Relation)
	return
}

type messageContent struct {
	Content     string
	ContentHTML string
}

type message struct {
	ID                  int
	Type                int
	TargetID            string
	MessageType         string
	MessageUID          string
	IsPersited          bool
	IsCounted           bool
	IsStatusMessage     bool
	SenderUserID        string
	Content             messageContent
	SentTime            int
	ReceivedTime        int
	MessageDirection    int
	IsOffLineMessage    bool
	DisableNotification bool
	CanIncludeExpansion bool
	Expansion           interface{}
}

// message sent
func (mes *message) send() error {
	mes.Content.ContentHTML = string(markdown.ToHTML([]byte(mes.Content.Content), nil, nil))
	content, err := json.Marshal(&mes.Content)
	checkErr(err)
	relationCur := userRelation{}
	relationCur.SubjectID = mes.SenderUserID
	relationCur.ObjectID = mes.TargetID
	relationCur.query()
	if mes.TargetID == "" {
		err = errors.New("No target ID for the message")
	} else if relationCur.Relation == 1 {
		_, err = db.Exec(`INSERT INTO message (Type, TargetID, MessageType, MessageUID, IsPersited, IsCounted, IsStatusMessage, SenderUserID, Content, SentTime, ReceivedTime, MessageDirection, IsOffLineMessage, DisableNotification, CanIncludeExpansion, Expansion) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16);`, mes.Type, mes.TargetID, mes.MessageType, mes.MessageUID, mes.IsPersited, mes.IsCounted, mes.IsStatusMessage, mes.SenderUserID, string(content), mes.SentTime, mes.ReceivedTime, mes.MessageDirection, mes.IsOffLineMessage, mes.DisableNotification, mes.CanIncludeExpansion, mes.Expansion)
	} else {
		err = errors.New("you're not friends")
	}
	return err
}

// message read
func (mes *message) read() error {
	updatePre, err := db.Prepare(`UPDATE message SET IsCounted=$1 WHERE id =$2;`)
	if err != nil {
		panic(err)
	}
	_, err = updatePre.Exec(false, mes.ID)
	return err
}

// message edit
func (mes *message) edit() (err error) {
	que := db.QueryRow(`SELECT senttime FROM message WHERE MessageUID=$1;`, mes.MessageUID)
	var senttime int
	que.Scan(&senttime)
	if time.Now().UnixNano()/int64(time.Millisecond)-int64(senttime) < 300000 {
		mes.Content.ContentHTML = string(markdown.ToHTML([]byte(mes.Content.Content), nil, nil))
		content, err := json.Marshal(&mes.Content)
		checkErr(err)
		_, err = db.Exec(`UPDATE message SET Content=$1 WHERE MessageUID=$2;`, content, mes.MessageUID)
		checkErr(err)
	} else {
		err = errors.New("Cannot edit since it's more than 5 minute-old")
	}
	return err
}

// message recall
func (mes *message) recall() (err error) {
	que := db.QueryRow(`SELECT senttime FROM message WHERE MessageUID=$1;`, mes.MessageUID)
	var senttime int
	que.Scan(&senttime)
	if time.Now().UnixNano()/int64(time.Millisecond)-int64(senttime) < 300000 {
		_, err = db.Exec(`DELETE FROM message WHERE MessageUID=$1;`, mes.MessageUID)
		checkErr(err)
	} else {
		err = errors.New("Cannot recall since it's more than 5 minute-old")
	}
	return err
}

type messageRes struct {
	Type                int
	TargetID            string
	MessageType         string
	MessageUID          string
	IsPersited          bool
	IsCounted           bool
	IsStatusMessage     bool
	SenderUserID        string
	Content             messageContent
	SentTime            int
	ReceivedTime        int
	MessageDirection    int
	IsOffLineMessage    bool
	DisableNotification bool
	CanIncludeExpansion bool
	Expansion           interface{}
}

type conversation struct {
	ID                 int
	SenderUserID       string
	UnreadMessageCount int
	HasMentiond        bool
	MentiondInfo       mentionedList
	LastUnreadTime     int
	NotificationStatus int
	IsTop              int
	Type               int
	TargetID           string
	LatestMessage      message
	HasMentioned       bool
	MentionedInfo      mentionedList
	UpdateTime         int
	Messages           []message
}

func (con *conversation) query() (err error) {
	targetCon := db.QueryRow(`SELECT ID, LatestMessage, UnreadMessageCount, HasMentiond, MentiondInfo, LastUnreadTime, NotificationStatus, IsTop, Type, HasMentioned, MentionedInfo FROM conversation WHERE SenderUserID=$1 AND TargetID=$2;`, con.SenderUserID, con.TargetID)
	targetCon.Scan(&con.ID, &con.LatestMessage, &con.UnreadMessageCount, &con.HasMentiond, &con.MentiondInfo, &con.LastUnreadTime, &con.NotificationStatus, &con.IsTop, &con.Type, &con.HasMentioned, &con.MentionedInfo)
	return err
}

func (con *conversation) read() error {
	_, err := db.Exec(`UPDATE conversation SET unreadMessageCount=0 WHERE SenderUserID=$1 AND TargetID=$2;`, con.SenderUserID, con.TargetID)
	checkErr(err)
	_, err = db.Exec(`UPDATE message SET IsCounted=$1 WHERE TargetID=$2;`, false, con.SenderUserID)
	checkErr(err)
	return err
}

func (con *conversation) update() error {
	conQuery, conQuery2 := conversation{}, conversation{}
	conQuery.SenderUserID, conQuery.TargetID = con.SenderUserID, con.TargetID
	conQuery2.TargetID, conQuery2.SenderUserID = con.SenderUserID, con.TargetID
	err := conQuery.query()
	checkErr(err)
	err = conQuery2.query()
	checkErr(err)

	latestMes, err := json.Marshal(&con.LatestMessage)
	checkErr(err)
	mentiondInfo, err := json.Marshal(&con.MentiondInfo)
	checkErr(err)
	mentionedInfo, err := json.Marshal(&con.MentionedInfo)
	checkErr(err)

	if conQuery.ID == 0 {
		_, err = db.Exec(`INSERT INTO conversation (SenderUserID, LatestMessage, UnreadMessageCount, HasMentiond, MentiondInfo, LastUnreadTime, NotificationStatus, IsTop, Type, TargetID, HasMentioned, MentionedInfo, UpdateTime) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);`, con.SenderUserID, latestMes, con.UnreadMessageCount, con.HasMentiond, mentiondInfo, con.LastUnreadTime, con.NotificationStatus, con.IsTop, con.Type, con.TargetID, con.HasMentioned, mentionedInfo, con.LatestMessage.ReceivedTime)
	} else {
		// UPDATE
		_, err = db.Exec(`UPDATE conversation SET UnreadMessageCount=$1, HasMentiond=$2, MentiondInfo=$3, LastUnreadTime=$4, NotificationStatus=$5, IsTop=$6, Type=$7, HasMentioned=$8, MentionedInfo=$9, LatestMessage=$10, UpdateTime=$11 WHERE id =$12;`, con.UnreadMessageCount, con.HasMentiond, mentiondInfo, con.LastUnreadTime, con.NotificationStatus, con.IsTop, con.Type, con.HasMentioned, mentionedInfo, latestMes, con.LatestMessage.ReceivedTime, conQuery.ID)
	}

	if conQuery2.ID == 0 {
		// INSERT
		_, err = db.Exec(`INSERT INTO conversation (SenderUserID, LatestMessage, UnreadMessageCount, HasMentiond, MentiondInfo, LastUnreadTime, NotificationStatus, IsTop, Type, TargetID, HasMentioned, MentionedInfo, UpdateTime) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);`, con.TargetID, latestMes, con.UnreadMessageCount, con.HasMentiond, mentiondInfo, con.LastUnreadTime, con.NotificationStatus, con.IsTop, con.Type, con.SenderUserID, con.HasMentioned, mentionedInfo, con.LatestMessage.ReceivedTime)
	} else {
		// UPDATE
		_, err = db.Exec(`UPDATE conversation SET UnreadMessageCount=$1, HasMentiond=$2, MentiondInfo=$3, LastUnreadTime=$4, NotificationStatus=$5, IsTop=$6, Type=$7, HasMentioned=$8, MentionedInfo=$9, LatestMessage=$10, UpdateTime=$11 WHERE id =$12;`, con.UnreadMessageCount, con.HasMentiond, mentiondInfo, con.LastUnreadTime, con.NotificationStatus, con.IsTop, con.Type, con.HasMentioned, mentionedInfo, latestMes, con.LatestMessage.ReceivedTime, conQuery2.ID)
	}
	return err
}

func (con *conversation) queryMessages(r *http.Request) error {
	session := userSession{}
	err := session.getSessionFromRequest(r)
	checkErr(err)
	queRes, err := db.Query(`SELECT type, targetid, messagetype, messageuid, ispersited, iscounted, isstatusmessage, senderuserid, content, senttime, receivedtime, messagedirection, isofflinemessage, disablenotification, canincludeexpansion, expansion FROM message WHERE (TargetId=$1 AND SenderUserID=$2) OR (TargetId=$2 AND SenderUserID=$1) ORDER BY senttime DESC;`, con.TargetID, session.userinDB)
	checkErr(err)
	for queRes.Next() {
		var contentStr string
		messageCur := message{}
		queRes.Scan(&messageCur.Type, &messageCur.TargetID, &messageCur.MessageType, &messageCur.MessageUID, &messageCur.IsPersited, &messageCur.IsCounted, &messageCur.IsStatusMessage, &messageCur.SenderUserID, &contentStr, &messageCur.SentTime, &messageCur.ReceivedTime, &messageCur.MessageDirection, &messageCur.IsOffLineMessage, &messageCur.DisableNotification, &messageCur.CanIncludeExpansion, &messageCur.Expansion)

		err = json.Unmarshal([]byte(contentStr), &messageCur.Content)
		checkErr(err)
		con.Messages = append(con.Messages, messageCur)
	}
	return err
}

type mentionedList struct {
	Type       int
	UserIDList []string
}
