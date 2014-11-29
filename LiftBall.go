package main

import (
	"encoding/json"
	"gopkg.in/qml.v0"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	output   chan string = make(chan string) //channel waitin on the user to type something
	testing  bool        = true              // if true prints out when every method is accessed and other info useful for debugging
	conn     net.Conn
	ctrl     Control
	lastSync time.Time
)

const (
	directory = "LiftBall/" //directory where the server files are stored
)

//graphics interface struct
type Control struct {
	Root        qml.Object
	convstring  string
	filelist    string
	inputString string
}

//all file info
type FileInfo struct {
	FileName  string
	Filesize  int64
	File      []byte
	Timestamp time.Time
}

//message sent between computers
type Message struct {
	Kind  string //type of message ("CONNECT","SYNC","LIST","GETFILE","STORE","DELETE")
	Files []FileInfo
	File  FileInfo
	IP    string
	When  time.Time
}

func main() {
	//starting graphics
	qml.Init(nil)
	engine := qml.NewEngine()
	ctrl = Control{convstring: ""}
	ctrl.convstring = ""
	context := engine.Context()
	context.SetVar("ctrl", &ctrl)
	component, err := engine.LoadFile("liftBall.qml")
	if err != nil {
		log.Println("no file to load for ui")
		log.Println(err.Error())
		os.Exit(0)
	}
	win := component.CreateWindow(nil)
	ctrl.Root = win.Root()

	os.Mkdir(directory, 0777) //creates LiftBall directory if it doesn't exist
	service := os.Args[1] + ":12100"
	tcpAddr, err := net.ResolveTCPAddr("tcp", service)
	handleError(err, "ABORT", "main1")
	conn, err = net.DialTCP("tcp", nil, tcpAddr)
	handleError(err, "ABORT", "main2")

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	win.Show() //show window
	ctrl.updateText("Hello liftBall client. Type List to get a list of the files stored on the server, Delete followed by the name of the file to delete a file on the server, and Get followed by the name of the file to get a file from the server.")

	//introduceMyself(*enc)

	go send(*enc)
	go receive(*dec, *enc)

	win.Wait()
}

/*

	//////////////////////////////////////CLIENT ACTIONS//////////////////////////////////////

*/

/*
	SYNC
	syncronizes the client's file and server's files
	@param json. connection to the server
	@param []FileInfo the clients files and their info
*/
func syncFiles(enc json.Encoder, msg *Message) {
	if testing {
		log.Println("syncFiles")
	}
	newMessage := new(Message)
	files := msg.Files
	for _, value := range files {
		if !doesFileExist(value.FileName) {
			if lastSync.Before(value.Timestamp) {
				file := FileInfo{FileName: value.FileName}
				newMessage = createMessage("GETFILE", "", make([]FileInfo, 0), file, time.Now())
				enc.Encode(newMessage)
			} else {
				file := FileInfo{FileName: value.FileName}
				newMessage = createMessage("DELETE", "", make([]FileInfo, 0), file, time.Now())
				enc.Encode(newMessage)
			}
		}
	}
	lastSync = time.Now()
}

//receives message from server
func receive(dec json.Decoder, enc json.Encoder) {
	if testing {
		log.Println("receive")
	}
	msg := new(Message)
	for {
		if err := dec.Decode(msg); err != nil {
			handleError(err, "ABORT", "receive")
		}
		switch msg.Kind {
		case "SYNC":
			syncFiles(enc, msg)
		case "LIST":
			ctrl.updateList(msg.Files)
		case "GETFILE":
			ctrl.updateText("server has requested " + msg.File.FileName + " from this client")
			fileInfo := getFile(msg.File.FileName)
			msg = createMessage("STORE", "", make([]FileInfo, 0), fileInfo, time.Now())
			enc.Encode(msg)
		case "STORE":
			ctrl.updateText("storing " + msg.File.FileName)
			storeFile(msg.File.FileName, msg.File.File)
		case "DELETE":
			ctrl.updateText("deleting " + msg.File.FileName)
			deleteFile(msg.File.FileName)
		}
	}
}

//sends message to the server
func send(enc json.Encoder) {
	if testing {
		log.Println("send")
	}
	msg := new(Message)
	message := ""
	for {
		sync := time.After(60000 * time.Millisecond)
		select {
		case message = <-output:
		case <-sync:
			message = "SYNC"
		}
		whatever := strings.Split(message, " ")
		var err error

		if strings.ToUpper(whatever[0]) == "SYNC" {
			msg = createMessage("SYNC", getMyIp(), getAllFiles(), FileInfo{}, time.Now())
		} else if strings.ToUpper(whatever[0]) == "DELETE" {
			log.Println(whatever[0], "sakjdas", whatever[1])
			msg = createMessage("DELETE", getMyIp(), getAllFiles(), FileInfo{FileName: whatever[1]}, time.Now())
		} else if strings.ToUpper(whatever[0]) == "LIST" {
			msg = createMessage("LIST", getMyIp(), make([]FileInfo, 0), FileInfo{}, time.Now())
		} else if strings.ToUpper(whatever[0]) == "STORE" {
			file, err := ioutil.ReadFile(directory + whatever[1])
			handleError(err, "PRINT", "send1")
			msg = createMessage("STORE", getMyIp(), make([]FileInfo, 0), FileInfo{FileName: whatever[1], File: file}, time.Now())
		}
		err = enc.Encode(msg)
		handleError(err, "PRINT", "send2")
	}
}

/*
	GETMYIP
	gets my ip
	@return string IP clients IP
*/
func getMyIp() (IP string) {
	if testing {
		log.Println("getMyIp")
	}
	name, err := os.Hostname()
	handleError(err, "PRINT", "getMyIp")
	addr, err := net.ResolveIPAddr("ip", name)
	handleError(err, "PRINT", "getMyIp")
	IP = addr.String()
	return
}

/*

	//////////////////////////////////////HANDELING FILES//////////////////////////////////////

*/

/*
	GETALLSERVERFILES
	gets all the files stored on the server
	@return []FileInfo slice containg info about all the files
	@return bool true if everything went fine, false if there was a problem
*/
func getAllFiles() []FileInfo {
	if testing {
		log.Println("getAllFiles")
	}
	allFiles, err := ioutil.ReadDir(directory)
	files := make([]FileInfo, 0, 10)
	for _, value := range allFiles {
		file := FileInfo{}
		file.FileName = value.Name()
		file.Filesize = value.Size()
		file.Timestamp = value.ModTime()
		files = append(files, file)
	}
	handleError(err, "PRINT", "getAllFiles")
	return files
}

/*
	GET
	gets the specified file with all its info.
	@param string fileName name of the file
	@return *FileInfo a pointer to a FileInfo struct
*/
func getFile(fileName string) (file FileInfo) {
	if testing {
		log.Println("getFile")
	}
	file = FileInfo{}
	var err error
	file.File, err = ioutil.ReadFile(directory + fileName)
	file.FileName = fileName
	fileStat, _ := os.Stat(directory + fileName)
	file.Filesize = fileStat.Size()
	file.Timestamp = fileStat.ModTime()
	handleError(err, "PRINT", "getFile")
	return
}

/*
	STORE
	stores a specific file on the client
	@param string fileName the name of the file to store
	@param []byte file the content of the file
	@return bool true if no error was ecnountered
*/
func storeFile(fileName string, file []byte) {
	if testing {
		log.Println("storeFile")
	}
	err := ioutil.WriteFile(directory+fileName, file, 0777)
	handleError(err, "PRINT", "storeFile")
}

/*
	DELETE
	deletes a file from the client
	@param string fileName the name of the file to delete
	@return bool false if something went wrong, true if everything went fine
*/
func deleteFile(fileName string) {
	if testing {
		log.Println("deleteFile")
	}
	handleError(os.Remove(directory+fileName), "PRINT", "deleteFile")
}

/*
	FILEEXISTS
	checks if a specific file exists on the client
	@param string fileName the name of the file to be checked
	@return bool true if the file exists, false if it doesn't
*/
func doesFileExist(fileName string) bool {
	if testing {
		log.Println("doesFileExist")
	}
	_, err := ioutil.ReadFile(directory + fileName)
	if err != nil {
		return false
	}
	return true
}

/*

	//////////////////////////////////////HANDELING MESSAGES//////////////////////////////////////

*/

/*
	CREATEMESSAGE
	@param string Kind the message kind
	@param string IP the IP of the sender. useless on the server
	@param []FileInfo files all the info of every file
	@param FileInfo file all the info of one file
	@return *Message a pointer to the created message
*/
func createMessage(Kind string, IP string, files []FileInfo, file FileInfo, t time.Time) (msg *Message) {
	if testing {
		log.Println("createMessage")
	}
	msg = new(Message)
	msg.Kind = Kind
	msg.IP = IP
	msg.Files = files
	msg.File = file
	msg.When = t
	return
}

/*

	//////////////////////////////////////HANDELING ERRORS//////////////////////////////////////

*/

/*
	HANDLEERROR
	checks if there's an error, if there is either prints it out or panics depending on action
	@param error err the error to check
	@param string action what to do if an error is found (ABORT, PRINT)
	@param string where where the error happened
*/
func handleError(err error, action string, where string) {
	if testing {
		log.Println("handleError " + where)
	}
	if err != nil {
		switch action {
		case "ABORT":
			panic("something went wrong in " + where)
		case "PRINT":
			ctrl.updateText("something went wrong in " + where)
		}
	}
}

/*

	//////////////////////////////////////GRAPHICS//////////////////////////////////////

*/

func (ctrl *Control) TextEntered(text qml.Object) {
	ctrl.inputString = text.String("text")
	output <- ctrl.inputString

}

func (ctrl *Control) updateText(toAdd string) {
	ctrl.convstring = ctrl.convstring + toAdd + "\n"
	ctrl.Root.ObjectByName("conv").Set("text", ctrl.convstring)
	qml.Changed(ctrl, &ctrl.convstring)
}

func (ctrl *Control) updateList(list []FileInfo) {
	ctrl.filelist = ""
	for _, file := range list {
		log.Println(file.Filesize)
		ctrl.filelist += "=========" + file.FileName + "=========\n" + "size: " + strconv.FormatInt(file.Filesize, 10) + "\n" + "last changed: " + file.Timestamp.Format(time.RFC1123) + "\n\n"
	}
	ctrl.Root.ObjectByName("filelist").Set("text", ctrl.filelist)
	qml.Changed(ctrl, &ctrl.filelist)
}
