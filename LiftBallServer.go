package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

var (
	testing        bool                 = true                      // if true prints out when every method is accessed and other info useful for debugging
	userLastAccess map[string]time.Time = make(map[string]time.Time) // keeps track of when any user accessed the server last
	mutex sync.Mutex
)

const (
	directory = "LiftBallBackup/" //directory where the server files are stored
)

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
	os.Mkdir(directory, 0777) //creates LiftBallBackup directory if it doesn't exist
	tcpAddr, err := net.ResolveTCPAddr("tcp4", ":12100")
	handleError(err, "ABORT", "main1")
	listener, err := net.ListenTCP("tcp", tcpAddr)
	handleError(err, "ABORT", "main2")
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go manageConnections(conn)
	}
}

/*
	MANAGE
	waits on a specific connection to send a message
	@param net.Conn connection to the client
*/
func manageConnections(conn net.Conn) {
	if testing {
		log.Println("handleClient")
	}
	defer conn.Close()
	dec := json.NewDecoder(conn)
	var msg Message
	for {
		err := dec.Decode(&msg)
		if err != nil {
			return
		}
		msg.handle(conn)
	}
}

/*

	//////////////////////////////////////SERVER ACTIONS//////////////////////////////////////

*/

/*
	SYNC
	syncronizes the client's file and server's files
	@param net.Conn connection to the client
	@param []FileInfo the clients files and their info
*/
func syncFiles(conn net.Conn, msg *Message) {
	if testing {
		log.Println("syncFiles")
	}
	files := msg.Files
	var newMessage = new(Message)
	for _, value := range files {
		if doesFileExist(value.FileName) {
			if serverLastChanged(value.FileName, value.Timestamp) > 0 {
				fileContent := getFile(value.FileName)
				file := FileInfo{FileName: value.FileName, File: fileContent.File}
				newMessage = createMessage("STORE", "", make([]FileInfo, 0), file)
			} else if serverLastChanged(value.FileName, value.Timestamp) < 0 {
				file := FileInfo{FileName: value.FileName}
				newMessage = createMessage("GETFILE", "", make([]FileInfo, 0), file)
			}
		} else {
			access, ok := userLastAccess[msg.IP]
			if !ok || access.Before(value.Timestamp) {
				file := FileInfo{FileName: value.FileName}
				newMessage = createMessage("GETFILE", "", make([]FileInfo, 0), file)
			} else {
				file := FileInfo{FileName: value.FileName}
				newMessage = createMessage("DELETE", "", make([]FileInfo, 0), file)
			}
		}
		newMessage.sendMessage(conn)
	}
	log.Println("send sync to client", getAllFiles())
		createMessage("SYNC", "", getAllFiles(), FileInfo{}).sendMessage(conn)
	userLastAccess[msg.IP] = msg.When
}

/*

	//////////////////////////////////////HANDELING MESSAGES//////////////////////////////////////

*/

/*
	HANDLE
	method of the Message struct
	handles a message coming from a user
	@param net.Conn connection to handle
*/
func (msg *Message) handle(conn net.Conn) {
	if testing {
		log.Println("handle")
	}
	switch msg.Kind {
	case "SYNC":
		syncFiles(conn, msg)
	case "LIST":
		files := getAllFiles()
		msg = createMessage("LIST", "", files, FileInfo{})
		msg.sendMessage(conn)
	case "GETFILE":
		msg = createMessage("STORE", "", make([]FileInfo, 0), getFile(msg.File.FileName))
		msg.sendMessage(conn)
	case "STORE":
		storeFile(msg.File)
	case "DELETE":
		deleteFile(msg.File.FileName)
	}
}

/*
	SEND
	sends a message to a client
	method of the struct Message
	@param net.Conn connection to the client
	@return bool false if something went wrong, true if everything was fine
*/
func (msg *Message) sendMessage(conn net.Conn) {
	if testing {
		log.Println("sendMessage")
	}
	enc := json.NewEncoder(conn)
	enc.Encode(msg)
}

/*
	CREATEMESSAGE
	@param string Kind the message kind
	@param string IP the IP of the sender. useless on the server
	@param []FileInfo files all the info of every file
	@param FileInfo file all the info of one file
	@return *Message a pointer to the created message
*/
func createMessage(Kind string, IP string, files []FileInfo, file FileInfo) (msg *Message) {
	msg = new(Message)
	msg.Kind = Kind
	msg.IP = IP
	msg.Files = files
	msg.File = file
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
	mutex.Lock()
	defer mutex.Unlock()
	allFiles, err := ioutil.ReadDir(directory)
	files := make([]FileInfo, 0, 10)
	for _, value := range allFiles {
		file := FileInfo{}
		file.File, _ = ioutil.ReadFile(value.Name())
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
	mutex.Lock()
	defer mutex.Unlock()
	file = FileInfo{}
	var err error
	file.File, err = ioutil.ReadFile(directory + fileName)
	file.FileName = fileName
	fileStat, _ := os.Stat(directory + fileName)
	file.Filesize = fileStat.Size()
	file.Timestamp = fileStat.ModTime()
	handleError(err, "PRINT", "getFile "+fileName)
	return
}

/*
	STORE
	stores a specific file on the server
	@param string fileName the name of the file to store
	@param []byte file the content of the file
	@return bool true if no error was ecnountered
*/
func storeFile(file FileInfo) {
	mutex.Lock()
	defer mutex.Unlock()
	if testing {
		log.Println("storeFile "+file.FileName)
	}
	err := ioutil.WriteFile(directory+file.FileName, file.File, 0777)
	os.Chtimes(directory+file.FileName, file.Timestamp, file.Timestamp)
	handleError(err, "PRINT", "storeFile "+file.FileName)
}

/*
	DELETE
	deletes a file from the server
	@param string fileName the name of the file to delete
	@return bool false if something went wrong, true if everything went fine
*/
func deleteFile(fileName string) {
	if testing {
		log.Println("deleteFile "+fileName)
	}
	mutex.Lock()
	defer mutex.Unlock()
	handleError(os.Remove(directory+fileName), "PRINT", "deleteFile "+fileName)
}

/*
	FILEEXISTS
	checks if a specific file exists on the server
	@param string fileName the name of the file to be checked
	@return bool true if the file exists, false if it doesn't
*/
func doesFileExist(fileName string) bool {
	mutex.Lock()
	defer mutex.Unlock()
	_, err := ioutil.ReadFile(directory + fileName)
	if err != nil {
		return false
	}
	return true
}

/*
	SERVERLASTCHANGED
	checks if a specific file was last changed on the server or client
	@param string fileName name of the file
	@param time.Time timeStamp time the file was last modified on the client
	@return int 1 if the server last changed the file, -1 if the client did and 0 if the file wasn't changed
*/
func serverLastChanged(fileName string, timeStamp time.Time) int {
	if testing {
		log.Println("serverLastChanged")
	}
	mutex.Lock()
	defer mutex.Unlock()
	file, err := os.Stat(directory + fileName)
	handleError(err, "PRINT", "serverLastChanged")
	if timeStamp.Before(file.ModTime()) {
		return 1
	} else if timeStamp.Equal(file.ModTime()) {
		return 0
	} else {
		return -1
	}
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
	if err != nil {
		switch action {
		case "ABORT":
			panic("something went wrong in " + where)
		case "PRINT":
			log.Println("something went wrong in " + where)
		}
	}
}
