package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/websocket"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Do you want to (S)hare or (R)eceive files? (S/R): ")
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	if choice == "S" {
		shareFiles(reader)
	} else if choice == "R" {
		receiveFiles(reader)
	} else {
		fmt.Println("Invalid choice")
	}
}

func shareFiles(reader *bufio.Reader) {
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatalf("Dial error: %v", err)
	}
	defer conn.Close()

	_, key, err := conn.ReadMessage()
	if err != nil {
		log.Fatalf("Read error: %v", err)
	}
	fmt.Printf("Room key: %s\n", string(key))

	for {
		fmt.Print("Enter file path to share (or 'exit' to quit): ")
		filePath, _ := reader.ReadString('\n')
		filePath = strings.TrimSpace(filePath)

		if filePath == "exit" {
			break
		}

		file, err := os.Open(filePath)
		if err != nil {
			fmt.Printf("File open error: %v\n", err)
			continue
		}

		fileName := filepath.Base(filePath)
		conn.WriteMessage(websocket.TextMessage, []byte("FILE:"+fileName))

		buffer := make([]byte, 1024)
		for {
			n, err := file.Read(buffer)
			if err != nil && err != io.EOF {
				log.Printf("File read error: %v", err)
				break
			}
			if n == 0 {
				break
			}
			err = conn.WriteMessage(websocket.BinaryMessage, buffer[:n])
			if err != nil {
				log.Printf("Write error: %v", err)
				break
			}
		}
		file.Close()
		conn.WriteMessage(websocket.TextMessage, []byte("EOF"))
		fmt.Printf("File %s shared successfully!\n", fileName)
	}
	conn.WriteMessage(websocket.TextMessage, []byte("CLOSE"))
}

func receiveFiles(reader *bufio.Reader) {
	fmt.Print("Enter the room key to join: ")
	key, _ := reader.ReadString('\n')
	key = strings.TrimSpace(key)

	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws", RawQuery: "key=" + key}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatalf("Dial error: %v", err)
	}
	defer conn.Close()

	fmt.Println("Connected to room successfully. Waiting for file list...")

	var files []string
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Fatalf("Read error: %v", err)
		}
		message := string(msg)
		if strings.HasPrefix(message, "FILES:") {
			files = strings.Split(strings.TrimPrefix(message, "FILES:"), ",")
			break
		}
	}

	fmt.Println("Available files:")
	for i, file := range files {
		fmt.Printf("%d. %s\n", i+1, file)
	}

	fmt.Print("Enter the number of the file you want to download: ")
	fileNum, _ := reader.ReadString('\n')
	fileNum = strings.TrimSpace(fileNum)

	conn.WriteMessage(websocket.TextMessage, []byte("GET:"+fileNum))

	_, msg, _ := conn.ReadMessage()
	fileName := string(msg)

	fmt.Printf("Receiving file: %s\n", fileName)
	file, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("File create error: %v", err)
	}
	defer file.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error: %v", err)
			}
			break
		}
		if string(msg) == "EOF" {
			break
		}
		_, err = file.Write(msg)
		if err != nil {
			log.Fatalf("File write error: %v", err)
		}
	}
	fmt.Println("File received successfully!")
}