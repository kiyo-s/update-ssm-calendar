package main

import (
	"flag"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func main() {
	calendarName := flag.String("n", "", "Name of the calendar to fetch")
	flag.Parse()

	doc, err := getCalendar(calendarName)

	if err != nil {
		log.Printf("Error fetching calendar: %v\n", err)
		os.Exit(1)
	}

	log.Println("first page results")
	log.Printf("Name: %s, Type: %s", aws.ToString(doc.Document.Name), doc.Document.DocumentType)
}
