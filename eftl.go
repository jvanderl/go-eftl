package eftl
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

const EFTL_VERSION = "3.1.0   V7"
const EFTL_WS_PROTOCOL = "v1.eftl.tibco.com"
const OP_HEARTBEAT = 0
const OP_LOGIN = 1
const OP_WELCOME = 2
const OP_SUBSCRIBE = 3
const OP_SUBSCRIBED = 4
const OP_UNSUBSCRIBE = 5
const OP_UNSUBSCRIBED = 6
const OP_EVENT = 7
const OP_MESSAGE = 8
const OP_ACK = 9
const OP_ERROR = 10
const OP_DISCONNECT = 11
const OP_GOODBYE = 12
const SUBSCRIPTION_TYPE = ".s."


type Connection struct {
    ConnectOptions string
    AccessPointURL string
    WebSocket *websocket.Conn
    State int
    ClientID string
    ReconnectToken string
    Timeout int
    Heartbeat int
    MaxMessageSize int
    LastMessage int
    TimeoutCheck int
    Subscriptions []Subscription //todo struct
    SequenceCounter int
    SubscriptionCounter int
    ReconnectCounter int
    ReconnectAttempts int
    ReconnectTimer int
    IsReconnecting bool
    IsOpen bool
    QoS bool
    SendList map[string]string // todo struct
    LastSequenceNumber int
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
	MaxMessageSize int `json:"max_size"`
	QoS string `json:"_qos"`
}

type eftlSubscriptionRequest struct {
	Operator int  `json:"op"`
	SubscriptionID string `json:"id"`	
	Matcher string `json:"matcher,omitempty"`
	Durable string `json:"durable,omitempty"`
}
type eftlSubscriptionResponse struct {
	Operator int  `json:"op"`
	SubscriptionID string `json:"id"`	
}

type eftlUnsubscriptionResponse struct {
	Operator int  `json:"op"`
	SubscriptionID string `json:"id"`	
	Error int `json:"err"`
	Reason string `json:"reason"`
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
	Sequence int `json:"seq",omitempty`
} 

type eftlAckMsg struct {
	Operator int  `json:"op"`
	Sequence int `json:"seq"`
}

type Subscription struct {
	ID string `json:"subscription_id"`
	Matcher string `json:"matcher"`
	Durable string `json:"durable"`
	Pending bool `json:"pending"`
}

func Connect(server string, channel string, options string) (conn Connection, err error) {

	wsURL := url.URL{Scheme: "ws", Host: server, Path: channel}

	// init eftlConnection
    conn.ConnectOptions = options
    conn.AccessPointURL = wsURL.String()
    conn.State = STATE_OPENING
    conn.Timeout = 600000
    conn.Heartbeat = 240000
    conn.MaxMessageSize = 0
    conn.LastMessage = 0
    conn.SequenceCounter = 0
    conn.SubscriptionCounter = 0
    conn.ReconnectCounter = 0
    conn.ReconnectAttempts = 5
    conn.IsReconnecting = false
    conn.IsOpen = false
    conn.QoS = true
    conn.SendList = nil // todo struct
    conn.LastSequenceNumber = 0

    // Connect to eftl Server
	conn.WebSocket, _, err = websocket.DefaultDialer.Dial(conn.AccessPointURL, nil)
	if err != nil {
		return conn, err
	}
	return conn, nil
}


func (conn *Connection) Login (user string, password string)  (err error){
	
	loginMessage := eftlLoginRequest{OP_LOGIN, "js", EFTL_VERSION, user, password, map[string]string{"_qos": "true"}}

	loginb, err := json.Marshal(loginMessage)
	if err != nil {
		return err
	}
	
	err = conn.WebSocket.WriteMessage(websocket.TextMessage, loginb)
	if err != nil {
		return err
	}

	msg, op := conn.GetMessage()
	switch op {
		case OP_WELCOME : { 
    		res := eftlLoginResponse{}
		    if err := json.Unmarshal(msg, &res); err != nil {
				return err
			}
			conn.ClientID = res.ClientID
			conn.ReconnectToken = res.IDToken
			conn.State = STATE_OPEN
			conn.Timeout = res.Timeout
			conn.Heartbeat = res.Heartbeat
			conn.MaxMessageSize = res.MaxMessageSize
			conn.QoS, _ = strconv.ParseBool(res.QoS)
			return nil
		}
		default: {
		}
	}
	err = errors.New("No login response received")
	return err
}

func (conn *Connection) Subscribe (matcher string, durable string) (subscriptionid string, err error){

	subID := conn.ClientID + SUBSCRIPTION_TYPE + strconv.Itoa(conn.nextSubscriptionSequence()) + ""
    var subscription = Subscription{};
    subscription.ID = subID;
   	subscription.Matcher = matcher;
   	subscription.Durable = durable
    subscription.Pending = true;

	subscriptionMessage := eftlSubscriptionRequest{OP_SUBSCRIBE, subID, matcher, subscription.Durable}
	subscrb, err := json.Marshal(subscriptionMessage)
	if err != nil {
		return "", err
	}
	err = conn.WebSocket.WriteMessage(websocket.TextMessage, subscrb)
	if err != nil {
		return "", err
	}

	msg, op := conn.GetMessage()
	switch op {
		case OP_SUBSCRIBED : { // subscription response
    		res := eftlSubscriptionResponse{}
		    if err := json.Unmarshal(msg, &res); err != nil {
				return "", err
			}
			conn.Subscriptions =  append(conn.Subscriptions, subscription)
			return res.SubscriptionID, nil
		}
		case OP_UNSUBSCRIBED : {
    		res := eftlUnsubscriptionResponse{}
		    if err := json.Unmarshal(msg, &res); err != nil {
				return "", err
			}    		
    		err = errors.New(res.Reason)
    		return "", err
		}
		default: {
		}
	}
	err = errors.New("Other message received: " + convert(msg))
	return "", err
}

func (conn *Connection) GetMessage () (message []byte, operator int){
	for {
		messageType, p, err := conn.WebSocket.ReadMessage()
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

func (conn *Connection) SendMessage (message string, destination string) (err error) {
	var sequence int = conn.nextSequence()
	res := eftlOutboundMessage{}
	res.Operator = OP_MESSAGE
	res.Body.Destination = destination
	res.Body.Text = message
	res.Body.Number = sequence
	if conn.QoS {
		res.Sequence = sequence
	}
		
	msg, err := json.Marshal(res)
	if err != nil {
		return err
	}
	if (conn.MaxMessageSize > 0 && len(msg) > conn.MaxMessageSize) {
		err = errors.New("Message exceeds maximum message size of " + string(conn.MaxMessageSize))
		return err
	}
	if conn.State == STATE_OPEN {
		err = conn.WebSocket.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			return err
		} 
	}  else {
		err = errors.New("Connection not open")
		return err
	}
	return nil
}

func MessageDetails(message []byte) (text string, destination string, err error) {
	res := eftlInboundMessage{}
	if err := json.Unmarshal(message, &res); err != nil {
		return "","", err
	}
	return res.Body.Text, res.Body.Destination, nil
}

func (conn *Connection) nextSequence() int {
	conn.SequenceCounter += 1
	return conn.SequenceCounter
}

func (conn *Connection) nextSubscriptionSequence() int {
	conn.SubscriptionCounter += 1
	return conn.SubscriptionCounter
}

func convert(b []byte) string {
	n := len(b)
	return string(b[:n])
}
