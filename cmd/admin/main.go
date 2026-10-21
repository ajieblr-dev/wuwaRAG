// Command admin provides CLI management utilities for wuwaRAG users and administration.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ajieblr-dev/wuwaRAG/internal/auth"
	"github.com/ajieblr-dev/wuwaRAG/internal/config"
	"github.com/ajieblr-dev/wuwaRAG/internal/db"
)

func printUsage() {
	fmt.Print(`wuwaRAG Admin CLI Tool

Usage:
  admin <command> [options]

Commands:
  create-admin   Create a new administrator account
  list-users     List all registered users and their quota stats
  reset-quota    Reset question quota for a specific user

Examples:
  go run ./cmd/admin create-admin --username admin --email admin@solaris.org --password mysecret
  go run ./cmd/admin list-users
  go run ./cmd/admin reset-quota --user rover --count 0 --limit 5
`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	cfg := config.Load()

	database, err := db.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database at %s: %v", cfg.DBPath, err)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch command {
	case "create-admin":
		fs := flag.NewFlagSet("create-admin", flag.ExitOnError)
		username := fs.String("username", "", "Admin username")
		email := fs.String("email", "", "Admin email address")
		password := fs.String("password", "", "Admin password")
		_ = fs.Parse(os.Args[2:])

		if *username == "" || *email == "" || *password == "" {
			fmt.Println("Error: --username, --email, and --password flags are all required")
			fs.Usage()
			os.Exit(1)
		}

		hash, err := auth.HashPassword(*password)
		if err != nil {
			log.Fatalf("Failed to hash password: %v", err)
		}

		user, err := database.CreateUser(ctx, *username, *email, hash, "admin", -1)
		if err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}

		fmt.Printf("✅ Admin account created successfully!\n")
		fmt.Printf("   User ID:  %s\n", user.ID)
		fmt.Printf("   Username: %s\n", user.Username)
		fmt.Printf("   Email:    %s\n", user.Email)
		fmt.Printf("   Role:     %s (Exempt from question limits)\n", user.Role)

	case "list-users":
		users, err := database.ListUsers(ctx)
		if err != nil {
			log.Fatalf("Failed to list users: %v", err)
		}

		if len(users) == 0 {
			fmt.Println("No users registered yet.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tUSERNAME\tEMAIL\tROLE\tQUESTIONS USED\tLIMIT\tCREATED AT")
		for _, u := range users {
			limitStr := fmt.Sprintf("%d", u.QuestionLimit)
			if strings.ToLower(u.Role) == "admin" || u.QuestionLimit < 0 {
				limitStr = "Unlimited"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
				u.ID,
				u.Username,
				u.Email,
				u.Role,
				u.QuestionCount,
				limitStr,
				u.CreatedAt.Format("2006-01-02 15:04"),
			)
		}
		_ = w.Flush()

	case "reset-quota":
		fs := flag.NewFlagSet("reset-quota", flag.ExitOnError)
		identifier := fs.String("user", "", "Username or Email or User ID")
		count := fs.Int("count", 0, "New question count (default 0)")
		limit := fs.Int("limit", 5, "New question limit (default 5)")
		_ = fs.Parse(os.Args[2:])

		if *identifier == "" {
			fmt.Println("Error: --user flag is required")
			fs.Usage()
			os.Exit(1)
		}

		user, err := database.GetUserByUsernameOrEmail(ctx, *identifier)
		if err != nil {
			// Try by ID
			user, err = database.GetUserByID(ctx, *identifier)
			if err != nil {
				log.Fatalf("User not found: %s", *identifier)
			}
		}

		if err := database.ResetUserQuestionCount(ctx, user.ID, *count, *limit); err != nil {
			log.Fatalf("Failed to reset quota: %v", err)
		}

		fmt.Printf("✅ Quota successfully updated for user %q (%s):\n", user.Username, user.ID)
		fmt.Printf("   Questions Asked: %d\n", *count)
		fmt.Printf("   Question Limit:  %d\n", *limit)

	default:
		fmt.Printf("Unknown command: %q\n", command)
		printUsage()
		os.Exit(1)
	}
}
