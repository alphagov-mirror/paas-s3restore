package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	region = "eu-west-1"
)

type ParsedArgs struct {
	CommandName string
	Args        map[string]string
}

func parseTimestamp(timestamp string) (restoreTime time.Time) {

	i, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	return time.Unix(i, 0)

}

func printUsage(command string, usage func()) func() {
	fmt.Fprintf(os.Stderr, "usage: s3r <command> <args>\n")
	switch command {
	case "restore":
		return func() {
			fmt.Fprintf(os.Stderr, " restore   Restore bucket objects\n")
			usage()
		}
	case "list":
		return func() {
			fmt.Fprintf(os.Stderr, " list   List object versions. Not implemented\n")
			usage()
		}
	default:
		fmt.Fprintf(os.Stderr, " restore   Restore bucket objects\n list   List object versions\n")
		return nil
	}
}

func parseArguments() ParsedArgs {
	restoreCommand := flag.NewFlagSet("restore", flag.ExitOnError)
	bkt := restoreCommand.String("bucket", "", "Source bucket. Default none. Required.")
	ts := restoreCommand.String("timestamp", "", "Restore point in time in UNIX timestamp format. Required.")
	prx := restoreCommand.String("prefix", "", "Object prefix. Default none.")

	listCommand := flag.NewFlagSet("list", flag.ExitOnError)
	since := listCommand.String("since", "", "Not implemented")

	if len(os.Args) == 1 {
		printUsage("", func() {})
		os.Exit(2)
	}

	switch os.Args[1] {
	case "restore":
		if err := restoreCommand.Parse(os.Args[2:]); err == nil {
			if *bkt == "" || *ts == "" {
				restoreCommand.Usage = printUsage("restore", restoreCommand.PrintDefaults)
				restoreCommand.Usage()
				os.Exit(2)
			}
		} else {
			log.Fatal(err)
		}
		return ParsedArgs{
			CommandName: "restore",
			Args: map[string]string{
				"bucket":    *bkt,
				"timestamp": *ts,
				"prefix":    *prx,
			},
		}

	case "list":
		if err := listCommand.Parse(os.Args[2:]); err == nil {
			restoreCommand.Usage = printUsage("list", listCommand.PrintDefaults)
			restoreCommand.Usage()
			fmt.Println(*since)
			os.Exit(2)
		} else {
			log.Fatal(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "%q is not valid command.\n", os.Args[1])
		os.Exit(2)
	}
	return ParsedArgs{}
}

func main() {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		log.Fatal("failed to create session,", err)
	}

	svc := s3.New(sess)

	args := parseArguments()

	switch args.CommandName {
	case "restore":
		bucket := args.Args["bucket"]
		prefix := args.Args["prefix"]
		timestamp := args.Args["timestamp"]

		listVersionsParams := &s3.ListObjectVersionsInput{
			Bucket: aws.String(bucket),
			Prefix: aws.String(prefix),
		}

		listVersionResp, err := svc.ListObjectVersions(listVersionsParams)
		if err != nil {
			log.Fatal(err.Error())
		}

		restoreTime := parseTimestamp(timestamp)
		var restored map[string]bool
		restored = make(map[string]bool)
		for _, version := range listVersionResp.Versions {
			if _, ok := restored[*version.Key]; !ok {
				// Amazon S3 returns object versions in the order in which they were stored,
				// with the most recently stored returned first.
				if restoreTime.After(*version.LastModified) {
					fmt.Printf("Restoring...\n %s\n", version)

					copyParams := &s3.CopyObjectInput{
						Bucket:     aws.String(bucket),
						CopySource: aws.String(bucket + "/" + *version.Key + "?versionId=" + *version.VersionId),
						Key:        aws.String(*version.Key),
					}
					copyResp, err := svc.CopyObject(copyParams)
					if err != nil {
						log.Fatal(err.Error())

					}
					restored[*version.Key] = true
					fmt.Printf("Restored:\n %s\n", copyResp)
				}
			}
		}
	}
}
