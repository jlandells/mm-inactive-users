package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/buger/jsonparser"
)

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
	defaultPort   = "8065"
	defaultScheme = "http"
	defaultAge    = 180
	pageSize      = 2 // TODO Set this back to the default of 60 after testing!
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

func getTeamID(mattermostCon mmConnection, mmTeam string) (string, error) {
	DebugPrint("Retrieving Team ID for team: " + mmTeam)

	teamID := ""

	url := fmt.Sprintf("%s://%s:%s/api/v4/teams?name=%s", mattermostCon.mmScheme, mattermostCon.mmURL, mattermostCon.mmPort, url.QueryEscape(mmTeam))
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

	// Extract the body of the message
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		LogMessage(errorLevel, "Unable to extract body data from Mqattermost response")
		return "", err
	}

	// Parse the response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		LogMessage(errorLevel, "Failed to convert body data")
		return "", err
	}

	// Convert the data to a string
	mmTeamData, err := json.Marshal(result)
	if err != nil {
		LogMessage(errorLevel, "Unable to convert user data to string")
		log.Fatal(err)
	}

	teamID, err = jsonparser.GetString([]byte(mmTeamData), "id")
	if err != nil {
		LogMessage(errorLevel, "Unable to retrieve team ID for team: "+mmTeam+" Error: "+err.Error())
		return "", err
	}

	return teamID, nil
}

// EpochToDate converts an Epoch time to a string representation of the date.
func EpochToDate(epoch int64) string {
	t := time.Unix(epoch, 0)      // Convert Epoch to *time.Time
	return t.Format("02-01-2006") // Return date in DD-MM-YYYY format
}

// DaysAgo calculates how many days ago a date, represented by Epoch time, was.
func DaysAgo(epoch int64) int {
	now := time.Now()
	then := time.Unix(epoch, 0)
	daysAgo := now.Sub(then).Hours() / 24 // Calculate difference in hours and convert to days
	return int(daysAgo)
}

func callGetUsers(mattermostCon mmConnection, mmTeamID string, page int, usersMap map[string]User, age int) (bool, error) {
	DebugPrint("Getting users page: " + string(page))

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
		if lastActivityAge >= age {
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

func processUsers(mattermostCon mmConnection, mmTeam string, age int, dryrun bool) error {

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

		DebugPrint("Processed page: " + string(currentPage))
		currentPage++
	}

	LogMessage(infoLevel, "All users reviewed")

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
	var DryRunFlag bool
	var DebugFlag bool

	flag.StringVar(&MattermostURL, "url", "", "The URL of the Mattermost instance (without the HTTP scheme)")
	flag.StringVar(&MattermostPort, "port", "", "The TCP port used by Mattermost. [Default: "+defaultPort+"]")
	flag.StringVar(&MattermostScheme, "scheme", "", "The HTTP scheme to be used (http/https). [Default: "+defaultScheme+"]")
	flag.StringVar(&MattermostToken, "token", "", "The auth token used to connect to Mattermost")
	flag.StringVar(&MattermostTeam, "team", "", "*Required*.  The name of the Mattermost team")
	flag.IntVar(&MaxAge, "age", defaultAge, "The number of days a user must have been inactive to be deactivated.  [Default: "+string(defaultAge)+"]")
	flag.BoolVar(&DryRunFlag, "dry-run", false, "This tells the code to simply list the users to be deactivated, without making any changes.")
	flag.BoolVar(&DebugFlag, "debug", false, "Enable debug output")

	flag.Parse()

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

	DebugPrint("Parameters: MattermostURL=" + MattermostURL + " MattermostPort=" + MattermostPort + " MattermostScheme=" + MattermostScheme + " MattermostToken=" + MattermostToken + " MaxAge=" + string(MaxAge))
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

	err := processUsers(mattermostConenction, MattermostTeam, MaxAge, dryRunMode)

	if err != nil {
		LogMessage(errorLevel, "Processing failed.  Error: "+err.Error())
		os.Exit(2)
	}

}
