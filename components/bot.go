package components

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/textproto"
	"regexp"
	"strings"
	"time"
)

const CSTFormat = "Jan 2 15:04:05 CST"

func timestamp() string {
	return TimeStamp(CSTFormat)
}

func TimeStamp(format string) string {
	return time.Now().Format(format)
}

type Bot struct {
	Channel     string
	conn        net.Conn
	Credentials *OAuthCred
	MsgRate     time.Duration
	Name        string
	Port        string
	PrivatePath string
	Server      string
	startTime   time.Time
}

type OAuthCred struct {
	Password string `json:"password,omitempty"`
}

type TwitchBot interface {
	Connect()
	Disconnect()
	HandleChat() error
	JoinChannel()
	ReadCredentials() error
	Say(msg string) error
	Start()
}

func (bb *Bot) Connect() {
	var err error
	fmt.Printf("[%s] Connecting to %s... \n", timestamp(), bb.Server)

	bb.conn, err = net.Dial("tcp", bb.Server+":"+bb.Port)
	if err != nil {
		fmt.Printf("[%s] Error connecting to %s, retrying.\n", timestamp(), bb.Server)
		bb.Connect()
		return
	}
	fmt.Printf("[%s] Connected to %s!\n", timestamp, bb.Server)
	bb.startTime = time.Now()
}

func (bb *Bot) Disconnect() {
	bb.conn.Close()
	upTime := time.Now().Sub(bb.startTime).Seconds()
	fmt.Printf("[%s] Disconnected from %s! | Live for: %fs\n", timestamp(), bb.Server, upTime)
}

var msgRegex *regexp.Regexp = regexp.MustCompile(`^:(\w+)!\w+@\w+\.tmi\.twitch\.tv (PRIVMSG) #\w+(?: :(.*))?$`)

var cmdRegex *regexp.Regexp = regexp.MustCompile(`^!(\w+)\s?(\w+)?`)

func (bb *Bot) HandleChat() error {
	fmt.Printf("[%s] Watching #%s...\n", timestamp(), bb.Channel)

	tp := textproto.NewReader(bufio.NewReader(bb.conn))

	for {
		line, err := tp.ReadLine()
		if err != nil {
			bb.Disconnect()
			return errors.New("bb.bot.HandleChat: Failed to read line from channel. Disconnected.")
		}
		fmt.Printf("[%s] %s\n", timestamp(), line)

		if "PING :tmi.twitch.tv" == line {
			bb.conn.Write([]byte("PONG :tmi.twitch.tv\r\n"))
			continue
		} else {
			matches := msgRegex.FindStringSubmatch(line)
			if matches != nil {
				userName := matches[1]
				msgType := matches[2]

				switch msgType {
				case "PRIVMSG":
					msg := matches[3]
					fmt.Printf("[%s] %s: %s\n", timestamp(), userName, msg)

					&model{choices: []string{fmt.Sprintf("[%s] %s: %s\n", timestamp(), userName, msg)}}
					cmdMatches := cmdRegex.FindStringSubmatch(msg)
					if cmdMatches != nil {
						cmd := cmdMatches[1]

						if userName == bb.Channel {
							switch cmd {
							case "tbdown":
								fmt.Printf(
									"[%s] Shutdown command received. Shutting down now...\n",
									timestamp(),
								)
								bb.Disconnect()
								return nil
							default:
							}
						}
					}
				default:
				}
			}
		}
		time.Sleep(bb.MsgRate)
	}
}

func (bb *Bot) JoinChannel() {
	fmt.Printf("[%s] Joining #%s...\n", timestamp(), bb.Channel)
	bb.conn.Write([]byte("PASS " + bb.Credentials.Password + "\r\n"))
	bb.conn.Write([]byte("NICK " + bb.Name + "\r\n"))
	bb.conn.Write([]byte("JOIN " + bb.Channel + "\r\n"))

	fmt.Printf("[%s] Joined #%s as @%s!\n", timestamp(), bb.Channel, bb.Name)
}

func (bb *Bot) ReadCredentials() error {
	credFile, err := ioutil.ReadFile(bb.PrivatePath)
	if err != nil {
		return err
	}

	bb.Credentials = &OAuthCred{}

	dec := json.NewDecoder(strings.NewReader(string(credFile)))
	if err = dec.Decode(bb.Credentials); err != nil && io.EOF != err {
		return err
	}

	return nil
}

func (bb *Bot) Say(message string) error {
	if "" == message {
		return errors.New("Bot.Say: message was empty")
	}

	_, err := bb.conn.Write([]byte(fmt.Sprintf("PRIVMSG #%s\r\n", bb.Channel, message)))
	fmt.Printf("[%s] Message: %s\n", timestamp(), message)
	if err != nil {
		return err
	}
	return nil
}

func (bb *Bot) Start() {
	err := bb.ReadCredentials()
	if err != nil {
		fmt.Println(err)
		fmt.Println("Aborting...")
		return
	}

	for {
		bb.Connect()
		bb.JoinChannel()
		err = bb.HandleChat()
		if err != nil {
			time.Sleep(1000 * time.Millisecond)
			fmt.Println(err)
			fmt.Println("Starting Bot again...")
		} else {
			return
		}
	}
}
