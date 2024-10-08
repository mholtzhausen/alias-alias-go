package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	bolt "go.etcd.io/bbolt"
)

var db *bolt.DB

func main() {
	var err error
	db, err = bolt.Open("cmdex.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("commands"))
		return err
	})
	if err != nil {
		log.Fatal(err)
	}

	var rootCmd = &cobra.Command{
		Use:   "cmdex",
		Short: "A CLI tool to store and execute custom commands",
		Long:  `cmdex allows users to store and execute custom commands or multi-step command sequences using short, memorable aliases.`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				runCommand(args[0], args[1:])
			} else {
				cmd.Help()
			}
		},
	}

	rootCmd.AddCommand(saveCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(editCmd())
	rootCmd.AddCommand(runCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func saveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "save <alias> <command>",
		Short: "Save a command set with an alias",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			alias := args[0]
			command := strings.Join(args[1:], " ")
			err := db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("commands"))
				return b.Put([]byte(alias), []byte(command))
			})
			if err != nil {
				fmt.Printf("Error saving command: %v\n", err)
			} else {
				fmt.Printf("Command saved with alias: %s\n", alias)
			}
		},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all saved aliases and their associated commands",
		Run: func(cmd *cobra.Command, args []string) {
			err := db.View(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("commands"))
				return b.ForEach(func(k, v []byte) error {
					fmt.Printf("%s: %s\n", k, v)
					return nil
				})
			})
			if err != nil {
				fmt.Printf("Error listing commands: %v\n", err)
			}
		},
	}
}

func editCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <alias> <new_command>",
		Short: "Edit an existing command set",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			alias := args[0]
			newCommand := strings.Join(args[1:], " ")
			err := db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("commands"))
				if b.Get([]byte(alias)) == nil {
					return fmt.Errorf("alias not found")
				}
				return b.Put([]byte(alias), []byte(newCommand))
			})
			if err != nil {
				fmt.Printf("Error editing command: %v\n", err)
			} else {
				fmt.Printf("Command updated for alias: %s\n", alias)
			}
		},
	}
}

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <alias> [args...]",
		Short: "Run a saved command set",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runCommand(args[0], args[1:])
		},
	}
}

func runCommand(alias string, args []string) {
	var command string
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("commands"))
		v := b.Get([]byte(alias))
		if v == nil {
			return fmt.Errorf("alias not found")
		}
		command = string(v)
		return nil
	})
	if err != nil {
		fmt.Printf("Error retrieving command: %v\n", err)
		return
	}

	// Replace placeholders with arguments
	for i, arg := range args {
		placeholder := fmt.Sprintf("$%d", i+1)
		command = strings.ReplaceAll(command, placeholder, arg)
	}

	// Split the command into parts
	cmdParts := strings.Fields(command)
	if len(cmdParts) == 0 {
		fmt.Println("Empty command")
		return
	}

	// Create the command
	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Error executing command: %v\n", err)
	}
}