package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/buger/jsonparser"
	"golang.org/x/term"
)

var Version = "development" // Default value - overwritten during bild process

var debugMode bool = false
var dryRunMode bool = false

// LogLevel is used to refer to the type of message that will be written using the logging code.
type LogLevel string

type mmConnection struct {
	mmURL    string
	mmPort   string
	mmScheme string
	mmToken  string
}

type User struct {
	UserID                string
	Username              string
	Email                 string
	FullName              string
	LastActivityOn        string
	DaysSinceLastActivity int
}

const (
	debugLevel   LogLevel = "DEBUG"
	infoLevel    LogLevel = "INFO"
	warningLevel LogLevel = "WARNING"
	errorLevel   LogLevel = "ERROR"
)

const (
	defaultPort           = "8065"
	defaultScheme         = "http"
	defaultAge            = 180
	pageSize              = 60
	defaultTerminalHeight = 24
)

// Logging functions

// LogMessage logs a formatted message to stdout or stderr
func LogMessage(level LogLevel, message string) {
	if level == errorLevel {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(os.Stdout)
	}
	log.SetFlags(log.Ldate | log.Ltime)
	log.Printf("[%s] %s\n", level, message)
}

// DebugPrint allows us to add debug messages into our code, which are only printed if we're running in debug more.
// Note that the command line parameter '-debug' can be used to enable this at runtime.
func DebugPrint(message string) {
	if debugMode {
		LogMessage(debugLevel, message)
	}
}

// getEnvWithDefaults allows us to retrieve Environment variables, and to return either the current value or a supplied default
func getEnvWithDefault(key string, defaultValue interface{}) interface{} {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}

// getTerminalHeight is a utility function used for pagination
func getTerminalHeight() int {
	fd := int(os.Stdout.Fd())
	if term.IsTerminal(fd) {
		_, height, err := term.GetSize(fd)
		if err == nil {
			return height
		}
	}
	return defaultTerminalHeight
}

// promptForKeypress is a utility function that displays a message and waits for a keypress.
// It takes 2 parameters:
// prompt: a string to be displayed to alert the users what they need to do
// allowedKeys: an array of strings for the keys that will be accepted.  Other keys will be ignored.
func promptForKeypress(prompt string, allowedKeys []string) (string, error) {

	DebugPrint("Waiting for keypress")

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print(prompt)
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		input = strings.TrimSpace(strings.ToUpper(input)) // Normalise the input

		for _, key := range allowedKeys {
			if input == strings.ToUpper(key) {
				return input, nil // Return the valid keypress
			}
		}

		fmt.Println("Invalid input.  Please try again.")
	}
}

// getTeamID translates the Mattermost Team name into the internal Team ID, which is required for other API calls.
// It takes 2 parameters:
// mattermostCon: the Mattermost connection details
// mmTeam: a string containing the name of the Mattermost Team
func getTeamID(mattermostCon mmConnection, mmTeam string) (string, error) {
	DebugPrint("Retrieving Team ID for team: " + mmTeam)

	teamID := ""

	url := fmt.Sprintf("%s://%s:%s/api/v4/teams/name/%s", mattermostCon.mmScheme, mattermostCon.mmURL, mattermostCon.mmPort, url.QueryEscape(mmTeam))
	DebugPrint("Teams lookup URL: " + url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		LogMessage(errorLevel, "Error preparing GET")
		return "", err
	}
	// Add the bearer token as a header
	req.Header.Add("Authorization", "Bearer "+mattermostCon.mmToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		LogMessage(errorLevel, "Failed to query Mattermost")
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		ListTeamsAndExit(mattermostCon)
	}
	if resp.StatusCode != 200 {
		LogMessage(errorLevel, "Call to Get Teams failed!  Returned HTTP status: "+resp.Status)
		os.Exit(4)
	}

	// Extract the body of the message
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		LogMessage(errorLevel, "Unable to extract body data from Mqattermost response")
		return "", err
	}

	teamID, err = jsonparser.GetString([]byte(body), "id")
	if err != nil {
		LogMessage(errorLevel, "Unable to retrieve team ID for team: "+mmTeam+" Error: "+err.Error())
		return "", err
	}

	return teamID, nil
}

// ListTeamsAndExit lists all available teams in Mattermost then exists.
// It's intended to be called if the supplied team isn't found
func ListTeamsAndExit(mattermostCon mmConnection) {
	DebugPrint("In ListTeamsAndExit")

	url := fmt.Sprintf("%s://%s:%s/api/v4/teams", mattermostCon.mmScheme, mattermostCon.mmURL, mattermostCon.mmPort)
	DebugPrint("Teams lookup URL: " + url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		LogMessage(errorLevel, "Error preparing GET")
		os.Exit(10)
	}
	// Add the bearer token as a header
	req.Header.Add("Authorization", "Bearer "+mattermostCon.mmToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		LogMessage(errorLevel, "Failed to query Mattermost")
		os.Exit(11)
	}
	defer resp.Body.Close()

	// Extract the body of the message
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		LogMessage(errorLevel, "Unable to extract body data from Mattermost response to Get Teams")
		os.Exit(12)
	}

	// Check if the response body is empty (indicating no more items)
	if string(body) == "[]" {
		LogMessage(errorLevel, "No Teams data returned from Mattermost!")
		os.Exit(13)
	}

	// Parse the response to extract the teams
	fmt.Printf("\n\nTeams available in Mattermost (internal name in brackets):\n\n")

	_, _ = jsonparser.ArrayEach(body, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		displayName, _ := jsonparser.GetString(value, "display_name")
		internalName, _ := jsonparser.GetString(value, "name")
		fmt.Printf(" - %s (%s)\n", displayName, internalName)
	})

	fmt.Printf("\n\nPlease ensure that one of these teams is present in your command-line\n\n")
	os.Exit(99)
}

// EpochToDate converts an Epoch time to a string representation of the date.
func EpochToDate(epoch int64) string {
	t := time.Unix(epoch/1000, 0) // Convert Epoch to *time.Time
	return t.Format("02-01-2006") // Return date in DD-MM-YYYY format
}

// DaysAgo calculates how many days ago a date, represented by Epoch time, was.
func DaysAgo(epoch int64) int {
	now := time.Now()
	then := time.Unix(epoch/1000, 0)
	daysAgo := now.Sub(then).Hours() / 24 // Calculate difference in hours and convert to days
	return int(daysAgo)
}

func callGetUsers(mattermostCon mmConnection, mmTeamID string, page int, usersMap map[string]User, age int) (bool, error) {
	DebugPrint("Getting users page: " + strconv.Itoa(page))

	// Construct the URL
	url := fmt.Sprintf("%s://%s:%s/api/v4/users?in_team=%s&sort=last_activity_at&per_page=%d&page=%d",
		mattermostCon.mmScheme, mattermostCon.mmURL, mattermostCon.mmPort,
		mmTeamID, pageSize, page)
	DebugPrint("Users lookup URL: " + url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		LogMessage(errorLevel, "Error preparing GET")
		return false, err
	}
	// Add the bearer token as a header
	req.Header.Add("Authorization", "Bearer "+mattermostCon.mmToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		LogMessage(errorLevel, "Failed to query Mattermost")
		return false, err
	}
	defer resp.Body.Close()

	// Extract the body of the message
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		LogMessage(errorLevel, "Unable to extract body data from Mattermost response")
		return false, err
	}

	// Check if the response body is empty (indicating no more items)
	if string(body) == "[]" {
		return false, nil // No more items
	}

	// Parse the items from the JSON and update the map
	_, err = jsonparser.ArrayEach(body, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		id, _ := jsonparser.GetString(value, "id")
		username, _ := jsonparser.GetString(value, "username")
		email, _ := jsonparser.GetString(value, "email")
		firstname, _ := jsonparser.GetString(value, "first_name")
		lastname, _ := jsonparser.GetString(value, "last_name")
		lastActivity, _ := jsonparser.GetInt(value, "last_activity_at")
		lastActivityAge := DaysAgo(lastActivity)
		deleteAt, _ := jsonparser.GetInt(value, "delete_at")
		roles, _ := jsonparser.GetString(value, "roles")
		if deleteAt > 0 {
			DebugPrint("Skipping user " + username + " - already disabled")
		}
		if strings.Contains(roles, "system_admin") {
			LogMessage(infoLevel, "Skipping "+username+", as they are a system admin")
		} else if lastActivityAge >= age && deleteAt == 0 {
			DebugPrint("Found user: " + username + " for deactivation")
			userFullname := fmt.Sprintf("%s %s", firstname, lastname)
			usersMap[id] = User{
				UserID:                id,
				Username:              username,
				Email:                 email,
				FullName:              userFullname,
				LastActivityOn:        EpochToDate(lastActivity),
				DaysSinceLastActivity: lastActivityAge}
		}
	})

	if err != nil {
		return false, err
	}

	return true, nil
}

func printAllIdentifiedUsers(Users map[string]User) {

	reader := bufio.NewReader(os.Stdin)
	pageSize := getTerminalHeight() - 1 // We're subtracting 1 to allow for the prompt line
	count := 2                          // Note that count starts at 2 to allow for the header lines

	fmt.Printf("\nIdentified Users\n================\n\n")
	for _, user := range Users {
		fmt.Printf("Username: %s, Email: %s, Full name: %s, Last Login: %s, Days Since Last Login: %d\n",
			user.Username, user.Email, user.FullName,
			user.LastActivityOn, user.DaysSinceLastActivity)

		count++

		if count%pageSize == 0 {
			fmt.Printf("Enter 'Q' to quit, or 'enter' key to continue...")
			input, _ := reader.ReadString('\n')
			input = strings.ToUpper(input)
			if input == "Q\n" || input == "Q\r\n" { // We're handling this for Linux/Mac and Windows alternatives
				break
			}
		}
	}
	fmt.Printf("\nTotal users identified: %d\n\n", len(Users))
}

// deactivateUsers is used to mark Mattermost users as inactive, or optionally delete them.
// It takes 3 parameters:
// - mmCon: the Mattermost conenction information
// - users: a map containing the users to be deactivated/deleted
// - hardDelete: a boolean value that determines if the users should be hard deleted.  The default is false (deactivate only).
func deactivateUsers(mmCon mmConnection, users map[string]User, hardDelete bool) error {

	if hardDelete {
		DebugPrint("Hard deleting users")
	} else {
		DebugPrint("Deactivating users")
	}

	for _, user := range users {
		var url string
		if hardDelete {
			// Delete permanently
			url = fmt.Sprintf("%s://%s:%s/api/v4/users/%s?permanent=true", mmCon.mmScheme, mmCon.mmURL, mmCon.mmPort, user.UserID)
		} else {
			// Mark inactive
			url = fmt.Sprintf("%s://%s:%s/api/v4/users/%s", mmCon.mmScheme, mmCon.mmURL, mmCon.mmPort, user.UserID)
		}
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			LogMessage(warningLevel, "Error preparing API call for user: "+user.Username)
			continue
		}

		// Set request headers
		req.Header.Add("Authorization", "Bearer "+mmCon.mmToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			LogMessage(warningLevel, "DELETE request failed for user: "+user.Username)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			LogMessage(warningLevel, "REST call returned: '"+resp.Status+"' when attempting to deactivate/delete user: "+user.Username)
			continue
		}
	}

	LogMessage(infoLevel, "Deactivations/deletions complete")

	return nil
}

func processUsers(mattermostCon mmConnection, mmTeam string, age int, dryrun bool, hardDelete bool) error {

	DebugPrint("Processing users")

	// Before we can read the users, we need to convert the team name to an ID
	teamID, err := getTeamID(mattermostCon, mmTeam)
	if err != nil {
		log.Fatal(err)
	}
	if teamID == "" {
		log.Fatal("Fatal error retrieving Team ID from Mattermost")
	}

	// Now we can start processing users
	candidateUsersMap := make(map[string]User)

	currentPage := 0
	for {
		hasMore, err := callGetUsers(mattermostCon, teamID, currentPage, candidateUsersMap, age)
		if err != nil {
			log.Fatal("Error calling API: " + err.Error())
			break
		}

		if !hasMore {
			DebugPrint("No more users to process.")
			break
		}

		DebugPrint("Processed page: " + strconv.Itoa(currentPage))
		currentPage++
	}

	LogMessage(infoLevel, "All users reviewed")
	if len(candidateUsersMap) == 0 {
		LogMessage(infoLevel, "No users found that have been inactive for more than "+strconv.Itoa(age)+" days")
		return nil
	}

	if dryrun {
		LogMessage(infoLevel, "Running in dry-run mode.  Writing list of identified users to the terminal.")

		printAllIdentifiedUsers(candidateUsersMap)
	} else {
		prompt := fmt.Sprintf("%d users identified as inactive.  Deactivate them? (Y)es/(N)o/(L)ist: ", len(candidateUsersMap))
		allowedKeys := []string{"Y", "N", "L"}

	loop:
		for {
			keypress, err := promptForKeypress(prompt, allowedKeys)
			if err != nil {
				LogMessage(errorLevel, "Error processing user input.  Aborting.")
				os.Exit(4)
			}

			switch keypress {
			case "Y":
				LogMessage(infoLevel, "Deactivating users")
				deactivateUsers(mattermostCon, candidateUsersMap, hardDelete)
				break loop

			case "N":
				LogMessage(infoLevel, "Aborting")

				break loop

			case "L":
				printAllIdentifiedUsers(candidateUsersMap)
			}
		}
	}

	return nil
}

func main() {

	// Parse Command Line
	DebugPrint("Parsing command line")

	var MattermostURL string
	var MattermostPort string
	var MattermostScheme string
	var MattermostToken string
	var MattermostTeam string
	var MaxAge int
	var HardDeleteFlag bool
	var DryRunFlag bool
	var DebugFlag bool
	var VersionFlag bool

	flag.StringVar(&MattermostURL, "url", "", "The URL of the Mattermost instance (without the HTTP scheme)")
	flag.StringVar(&MattermostPort, "port", "", "The TCP port used by Mattermost. [Default: "+defaultPort+"]")
	flag.StringVar(&MattermostScheme, "scheme", "", "The HTTP scheme to be used (http/https). [Default: "+defaultScheme+"]")
	flag.StringVar(&MattermostToken, "token", "", "The auth token used to connect to Mattermost")
	flag.StringVar(&MattermostTeam, "team", "", "*Required*.  The name of the Mattermost team")
	MaxAgeDescription := fmt.Sprintf("The number of days a user must have been inactive to be deactivated.  [Default: %d]", defaultAge)
	flag.IntVar(&MaxAge, "age", defaultAge, MaxAgeDescription)
	flag.BoolVar(&HardDeleteFlag, "hard-delete", false, "Hard delete users, rather than just marking them as inactive.")
	flag.BoolVar(&DryRunFlag, "dry-run", false, "This tells the code to simply list the users to be deactivated, without making any changes.")
	flag.BoolVar(&DebugFlag, "debug", false, "Enable debug output")
	flag.BoolVar(&VersionFlag, "version", false, "Show version information and exit")

	flag.Parse()

	if VersionFlag {
		fmt.Printf("\nmm-inactive-users - Version: %s\n\n", Version)
		os.Exit(0)
	}

	// If information not supplied on the command line, check whether it's available as an envrionment variable
	if MattermostURL == "" {
		MattermostURL = getEnvWithDefault("MM_URL", "").(string)
	}
	if MattermostPort == "" {
		MattermostPort = getEnvWithDefault("MM_PORT", defaultPort).(string)
	}
	if MattermostScheme == "" {
		MattermostScheme = getEnvWithDefault("MM_SCHEME", defaultScheme).(string)
	}
	if MattermostToken == "" {
		MattermostToken = getEnvWithDefault("MM_TOKEN", "").(string)
	}
	if !DebugFlag {
		DebugFlag = getEnvWithDefault("MM_DEBUG", debugMode).(bool)
	}

	DebugMessage := fmt.Sprintf("Parameters: MattermostURL=%s, MattermostPort=%s, MattermostScheme=%s, MattermostToken=%s, MaxAge=%d",
		MattermostURL,
		MattermostPort,
		MattermostScheme,
		MattermostToken,
		MaxAge)
	DebugPrint(DebugMessage)
	if DryRunFlag {
		DebugPrint("Dry-run flag is set")
	}

	// Validate required parameters
	DebugPrint("Validating parameters")
	var cliErrors bool = false
	if MattermostURL == "" {
		LogMessage(errorLevel, "The Mattermost URL must be supplied either on the command line of vie the MM_URL environment variable")
		cliErrors = true
	}
	if MattermostScheme == "" {
		LogMessage(errorLevel, "The Mattermost HTTP scheme must be supplied either on the command line of vie the MM_SCHEME environment variable")
		cliErrors = true
	}
	if MattermostToken == "" {
		LogMessage(errorLevel, "The Mattermost auth token must be supplied either on the command line of vie the MM_TOKEN environment variable")
		cliErrors = true
	}
	if MattermostTeam == "" {
		LogMessage(errorLevel, "A Mattermost team name is required to use this utility.")
		cliErrors = true
	}
	if MaxAge < 30 {
		LogMessage(warningLevel, "The supplied age parameter is relatively low!  Please validate that the correct value was used prior to deactivating users!")
	}
	if cliErrors {
		flag.Usage()
		os.Exit(1)
	}

	debugMode = DebugFlag
	dryRunMode = DryRunFlag

	mattermostConenction := mmConnection{
		mmURL:    MattermostURL,
		mmPort:   MattermostPort,
		mmScheme: MattermostScheme,
		mmToken:  MattermostToken,
	}

	LogMessage(infoLevel, "Processing started - Version: "+Version)

	err := processUsers(mattermostConenction, MattermostTeam, MaxAge, dryRunMode, HardDeleteFlag)

	if err != nil {
		LogMessage(errorLevel, "Processing failed.  Error: "+err.Error())
		os.Exit(2)
	}

}
