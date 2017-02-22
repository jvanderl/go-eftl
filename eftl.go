package eftl

import (
	"github.com/gorilla/websocket"
	"encoding/json"
	"net/url"
	"errors"
	"strconv"
)

const STATE_OPENING int = 0
const STATE_OPEN int = 1
const STATE_CLOSING int = 2
const STATE_CLOSED int = 3



var log = logging.MustGetLogger("lib-go-eftl")

type eftlConnection struct {
    connectOptions string
    accessPointURL string
    webSocket websocket.Conn
    state int
    clientId string
    reconnectToken string
    timeout int
    heartbeat int
    maxMessageSize int
    lastMessage int
    timeoutCheck int
    subscriptions map[string]string //todo struct
    sequenceCounter int
    subscriptionCounter int
    reconnectCounter int
    reconnectAttempts int
    reconnectTimer int
    isReconnecting bool
    isOpen bool
    qos bool
    sendList map[string]string // todo struct
    lastSequenceNumber int
}

type eftlLoginRequest struct {
	Operator int  `json:"op"`
	ClientType string `json:"client_type"`
	ClientVersion string `json:"client_version"`
	User string `json:"user"`
	Password string `json:"password"`
	LoginOptions map[string]string `json:"login_options"`
}

type eftlLoginResponse struct {
	Operator int  `json:"op"`
	ClientID string `json:"client_id"`	
	IDToken string `json:"id_token"`
	Timeout int `json:"timeout"`
	Heartbeat int `json:"heartbeat"`
	MaxSize int `json:"max_size"`
	QoS string `json:"_qos"`
}


type eftlSubscriptionRequest struct {
	Operator int  `json:"op"`
	ClientID string `json:"id"`	
	Matcher string `json:"matcher"`
}

type eftlSubscriptionResponse struct {
	Operator int  `json:"op"`
	SubscriptionID string `json:"id"`	
}


type eftlBody struct {
	Destination string `json:"_dest"`
	Text string `json:"text"`
	Number int `json:"number"`
}

type eftlInboundMessage struct {
	Operator int  `json:"op"`
	To string `json:"to"`
	Sequence int `json:"seq"`
	Body eftlBody `json:"body"`
} 

type eftlOutboundMessage struct {
	Operator int  `json:"op"`
	Body eftlBody `json:"body"`
	Sequence int `json:"seq"`
} 

type eftlAckMsg struct {
	Operator int  `json:"op"`
	Sequence int `json:"seq"`
}

type eftlSubsctiption struct {
	SubscriptionID string

}

func Login (conn eftlConnection, user string, password string)  (err error){
	
	loginMessage := eftlLoginRequest{1, "js", "3.1.0   V7", user, password, map[string]string{"_qos": "true"}}

	loginb, err := json.Marshal(loginMessage)
	if err != nil {
		return err
	}
	
	err = conn.webSocket.WriteMessage(websocket.TextMessage, loginb)
	if err != nil {
		return err
	}

	msg, op := GetMessage (conn.webSocket)
	switch op {
		case 2 : { // Login response
    		res := new(eftlLoginResponse)
		    if err := json.Unmarshal(msg, &res); err != nil {
				return err
			}
			conn.clientId := res.ClientID
			conn.reconnectToken := res.IDToken 
			return nil
		}
		default: {
		}
	}
	err = errors.New("No login response received")
	return err
}

/*func Login (conn websocket.Conn, user string, password string) (clientid string, idtoken string, err error){
	
	loginMessage := eftlLoginRequest{1, "js", "3.1.0   V7", user, password, map[string]string{"_qos": "true"}}

	loginb, err := json.Marshal(loginMessage)
	if err != nil {
		return "", "", err
	}
	
	err = conn.WriteMessage(websocket.TextMessage, loginb)
	if err != nil {
		return "", "", err
	}

	msg, op := GetMessage (conn)
	switch op {
		case 2 : { // Login response
    		res := new(eftlLoginResponse)
		    if err := json.Unmarshal(msg, &res); err != nil {
				return "", "", err
			}
			return res.ClientID, res.IDToken, nil
		}
		default: {
		}
	}
	err = errors.New("No login response received")
	return "", "", err
}
*/

func Connect(server string, channel string, options *string) (conn *eftlConnection, err error) {

	//wsURL := url.URL{Scheme: "ws", Host: server, Path: channel}
	conn := eftConnection{}
    conn.connectOptions := options
    conn.accessPointURL := url.URL{Scheme: "ws", Host: server, Path: channel}
    conn.webSocket websocket.Conn
    conn.state := STATE_OPENING
    conn.clientId := nil
    conn.reconnectToken := nil
    conn.timeout := 600000
    conn.heartbeat := 240000
    conn.maxMessageSize := 0
    conn.lastMessage := 0
    conn.timeoutCheck := nil
    conn.subscriptions := nil
    conn.sequenceCounter := 0
    conn.subscriptionCounter := 0
    conn.reconnectCounter := 0
    conn.reconnectAttempts := 5
    conn.reconnectTimer := nil
    conn.isReconnecting := false
    conn.isOpen := false
    conn.qos  := true
    conn.sendList := nil // todo struct
    conn.lastSequenceNumber := 0
	conn.webSocket, _, err = websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

/*func Connect(server string, channel string) (conn *websocket.Conn, err error) {

	wsURL := url.URL{Scheme: "ws", Host: server, Path: channel}

	conn, _, err = websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}*/

func Subscribe (conn websocket.Conn, clientid string, index int, destination string) (subscriptionid string, err error){

	subscriptionMessage := eftlSubscriptionRequest{3, clientid + ".s." + strconv.Itoa(index) + "", "{\"_dest\":\"" + destination + "\"}"}

	subscrb, err := json.Marshal(subscriptionMessage)
	if err != nil {
		return "", err
	}
	
	err = conn.WriteMessage(websocket.TextMessage, subscrb)
	if err != nil {
		return "", err
	} 

	msg, op := GetMessage (conn)
	switch op {
		case 4 : { // subscription response
    		res := eftlSubscriptionResponse{}
		    if err := json.Unmarshal(msg, &res); err != nil {
				return "", err
			}
			return res.SubscriptionID, nil
		}
		default: {
		}
	}
	err = errors.New("No subscription response received")
	return "", err
}

func GetMessage (conn websocket.Conn) (message []byte, operator int){
	for {
		messageType, p, err := conn.ReadMessage()
		if err == nil {
			switch messageType {
		    	case websocket.TextMessage : {
		    		var dat map[string]interface{}
		    		if err := json.Unmarshal(p, &dat); err != nil {
     				   panic(err)
    				}
    				eftlOp := dat["op"].(float64)
    				if eftlOp != 0 { //Skip Heartbeat
    					return p, int(eftlOp)
    				}
		    	}
		    	case websocket.BinaryMessage : {
		    	}
		    	case websocket.CloseMessage : {
		    	}
		    	case websocket.PingMessage : {
		    	}
		    	case websocket.PongMessage : {
		    	}
		    }
		} 
	}
}

func SendMessage (conn websocket.Conn, message string, destination string)
	res := eftlOutboundMessage{}
	res.Operator := 8
	res.Body.Destination := destination
	res.Body.Text := message


func MessageDetails(message []byte) (text string, destination string, err error) {
	res := eftlInboundMessage{}
	if err := json.Unmarshal(message, &res); err != nil {
		return "","", err
	}
	return res.Body.Text, res.Body.Destination, nil
}

func convert(b []byte) string {
	n := len(b)
	return string(b[:n])
}
