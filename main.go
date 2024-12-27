package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/stdr"
	"golang.org/x/oauth2/google"

	"github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/config"
	"github.com/theupdateframework/go-tuf/v2/metadata/updater"
)

// The following config is used to fetch a target from Jussi's GitHub repository example
const (
	metadataURL          = "https://sorayaormazabalmayo.github.io/TUF_Repository_YubiKey_Vault/metadata"
	targetsURL           = "https://sorayaormazabalmayo.github.io/TUF_Repository_YubiKey_Vault/targets"
	verbosity            = 4
	generateRandomFolder = false
	nameOfFile           = "index.json"
)

type indexInfo struct {
	Length int64 `json:"length"`
	Hashes struct {
		Sha256 string `json:"sha256"`
	} `json:"hashes"`
	Version string `json:"version"`
}

func main() {

	// Define the desired layout
	layout := "2006.01.02-15.04.05"

	// This is the first step for setting the initial configuration.

	// set logger to stdout with info level
	metadata.SetLogger(stdr.New(stdlog.New(os.Stdout, "client_example", stdlog.LstdFlags)))
	stdr.SetVerbosity(verbosity)

	log := metadata.GetLogger()

	// initialize environment - temporary folders, etc.
	metadataDir, err := InitEnvironment()
	if err != nil {
		log.Error(err, "Failed to initialize environment")
	}

	// initialize client with Trust-On-First-Use
	err = InitTrustOnFirstUse(metadataDir)
	if err != nil {
		log.Error(err, "Trust-On-First-Use failed")
	}

	// Download the target index considering trusted targets role
	targetIndexFile, foundDesiredTargetIndexLocally, err := DownloadTargetIndex(metadataDir)

	if err != nil {
		log.Error(err, "Download index file failed")
	}

	if foundDesiredTargetIndexLocally == 0 {

		err = os.WriteFile(filepath.Join(metadataDir, nameOfFile), targetIndexFile, 0750)
		if err != nil {
			log.Error(err, "Error writing to file")
		}

	} else {
		fmt.Printf("\nThe local index file is the most updated one \n")
	}

	// Getting the latest version of the desired file

	// Map to hold the top-level JSON keys
	var data map[string]indexInfo

	// Parse JSON into the map
	err = json.Unmarshal([]byte(targetIndexFile), &data)
	if err != nil {
		fmt.Printf("Error parsing JSON: %v", err)
	}
	// Latest version considering the index.json downloaded by TUF

	indexVersion := data["nebula-standalone"].Version

	//hashLatestVersion := data["nebula-standalone"].Hashes

	// Service account key file
	serviceAccountKeyPath := "/home/sormazabal/artifact-downloader-key.json"

	// Construct Artifact Registry URL
	url := fmt.Sprintf("https://artifactregistry.googleapis.com/download/v1/projects/polished-medium-445107-i9/locations/europe-southwest1/repositories/nebula-storage/files/nebula-package:%s:nebula-standalone:download?alt=media", indexVersion)

	fmt.Printf("Downloading binary from: %s\n", url)

	// Download the artifact without specifying the file type
	err = downloadArtifact(serviceAccountKeyPath, url)
	if err != nil {
		fmt.Printf("Failed to download binary: %v\n", err)
		os.Exit(1)
	}

	verficationAnswer := verifyingDownloadedFile(string(targetIndexFile), "tmp/downloaded-file")

	if verficationAnswer == 1 {
		fmt.Printf("\U0001F7E2Binary downloaded successfully!\U0001F7E2\n")
	} else {
		fmt.Printf("\U0001F534There has been an error while downloading the file. The hashed do not match\U0001F534\n")

	}

	currentVersion := data["nebula-standalone"].Version

	// Printing expiration date
	PrintExpirationDate(layout, currentVersion)

	fmt.Printf("\nThe current nebula-standalone version is: %s \n", currentVersion)

	time.Sleep(time.Second * 60)

	// The updater needs to be looking for new updates every x time
	for {
		// download the desired target
		targetIndexFile, foundDesiredTargetIndexLocally, err := DownloadTargetIndex(metadataDir)

		if err != nil {
			log.Error(err, "Download index file failed")
		}

		if foundDesiredTargetIndexLocally == 0 {

			err = os.WriteFile(filepath.Join(metadataDir, nameOfFile), targetIndexFile, 0750)
			if err != nil {
				log.Error(err, "Error writing to file")
			}

			// Verifying that the index.json's version is latest than the one that is currently running

			// Map to hold the top-level JSON keys
			var data map[string]indexInfo

			// Parse JSON into the map
			err = json.Unmarshal([]byte(targetIndexFile), &data)
			if err != nil {
				fmt.Printf("\U0001F534Error parsing JSON: %v\U0001F534", err)
			}
			// Latest version considering the index.json downloaded by TUF

			indexVersion := data["nebula-standalone"].Version

			newProductVersion := NewVersion(currentVersion, indexVersion, layout)

			if newProductVersion == 1 {
				fmt.Printf("There is a new product of nebula-standalone\n")
			} else {
				fmt.Printf("There is no new product\n")
			}

			// Getting user answer

			userAnswer := gettingUserAnswer()

			if userAnswer == 1 {

				//hashLatestVersion := data["nebula-standalone"].Hashes

				// Service account key file
				serviceAccountKeyPath := "/home/sormazabal/artifact-downloader-key.json"

				// Construct Artifact Registry URL
				url := fmt.Sprintf("https://artifactregistry.googleapis.com/download/v1/projects/polished-medium-445107-i9/locations/europe-southwest1/repositories/nebula-storage/files/nebula-package:%s:nebula-standalone:download?alt=media", indexVersion)

				fmt.Printf("Downloading binary from: %s\n", url)

				// Download the artifact without specifying the file type
				err = downloadArtifact(serviceAccountKeyPath, url)
				if err != nil {
					fmt.Printf("\U0001F534Failed to download binary: %v\U0001F534\n", err)
					os.Exit(1)
				}

				verficationAnswer := verifyingDownloadedFile(string(targetIndexFile), "tmp/downloaded-file")

				if verficationAnswer == 1 {
					fmt.Printf("\U0001F7E2Binary downloaded successfully!\U0001F7E2\n")
				} else {
					fmt.Printf("\U0001F534There has been an error while downloading the file. The hashed do not match\n\U0001F534")

				}

			} else {

				fmt.Printf("\u23F0Remember that you have an update pending.\u23F0\n")

				// Telling the user the expiration date of the current version

				PrintExpirationDate(layout, currentVersion)

			}

		} else {
			fmt.Printf("\nThe local index file is the most updated one\n")
		}

		time.Sleep(time.Second * 60)

	}
}

// InitEnvironment prepares the local environment - temporary folders, etc.
func InitEnvironment() (string, error) {
	var tmpDir string
	// get working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	if !generateRandomFolder {
		tmpDir = filepath.Join(cwd, "tmp")
		// create a temporary folder for storing the demo artifacts
		os.Mkdir(tmpDir, 0750)
	} else {
		// create a temporary folder for storing the demo artifacts
		tmpDir, err = os.MkdirTemp(cwd, "tmp")
		if err != nil {
			return "", fmt.Errorf("failed to create a temporary folder: %w", err)
		}
	}

	// create a destination folder for storing the downloaded target
	os.Mkdir(filepath.Join(tmpDir, "download"), 0750)
	return tmpDir, nil
}

// InitTrustOnFirstUse initialize local trusted metadata (Trust-On-First-Use)
func InitTrustOnFirstUse(metadataDir string) error {
	// check if there's already a local root.json available for bootstrapping trust
	_, err := os.Stat(filepath.Join(metadataDir, "root.json"))
	if err == nil {
		return nil
	}

	// download the initial root metadata so we can bootstrap Trust-On-First-Use
	rootURL, err := url.JoinPath(metadataURL, "1.root.json")
	if err != nil {
		return fmt.Errorf("failed to create URL path for 1.root.json: %w", err)
	}

	req, err := http.NewRequest("GET", rootURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	client := http.DefaultClient

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to executed the http request: %w", err)
	}

	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read the http request body: %w", err)
	}

	// write the downloaded root metadata to file
	err = os.WriteFile(filepath.Join(metadataDir, "root.json"), data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write root.json metadata: %w", err)
	}

	return nil
}

// DownloadTargetIndex downloads the target file using Updater. The Updater refreshes the top-level metadata,
// get the target information, verifies if the target is already cached, and in case it
// is not cached, downloads the target file.

func DownloadTargetIndex(localMetadataDir string) ([]byte, int, error) {
	// log := metadata.GetLogger()

	rootBytes, err := os.ReadFile(filepath.Join(localMetadataDir, "root.json"))
	if err != nil {
		return nil, 0, err
	}

	// create updater configuration
	cfg, err := config.New(metadataURL, rootBytes) // default config
	if err != nil {
		return nil, 0, err
	}
	cfg.LocalMetadataDir = localMetadataDir
	cfg.LocalTargetsDir = filepath.Join(localMetadataDir, "download")
	cfg.RemoteTargetsURL = targetsURL
	cfg.PrefixTargetsWithHash = true

	// create a new Updater instance
	up, err := updater.New(cfg)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create Updater instance: %w", err)
	}

	// try to build the top-level metadata
	err = up.Refresh()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to refresh trusted metadata: %w", err)
	}

	ti, err := up.GetTargetInfo(nameOfFile)
	if err != nil {
		return nil, 0, fmt.Errorf("getting info for target index \"%s\": %w", nameOfFile, err)
	}

	path, tb, err := up.FindCachedTarget(ti, filepath.Join(localMetadataDir, nameOfFile))
	if err != nil {
		return nil, 0, fmt.Errorf("getting target index cache: %w", err)
	}

	// fmt.Printf("\n%s\n", tb)
	// fmt.Printf("\n%s\n", path)

	if path != "" {
		// Cached version found
		fmt.Println("\U0001F34C CACHE HIT")
		return tb, 1, nil
	}

	// fmt.Printf("\nThere is a new update:\n")

	// Download of target is needed
	_, tb, err = up.DownloadTarget(ti, "", "")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to download target index file %s - %w", nameOfFile, err)
	}

	return tb, 0, nil
}

func gettingUserAnswer() int {

	var userAnswer int

	fmt.Printf("\n Do you want to download the new version?\n")

	fmt.Printf("\n Introduce your answer: \n")
	fmt.Println("------------------------------------------")
	fmt.Printf("\nFor YES => (1)")
	fmt.Printf("\nFor NO  => (2)\n")

	fmt.Scanf("%d", &userAnswer)

	return userAnswer

}

// downloadArtifact dynamically determines the file name and downloads the artifact
func downloadArtifact(keyFilePath, url string) error {
	// Authenticate using the service account key
	ctx := context.Background()
	creds, err := google.CredentialsFromJSON(ctx, readFile(keyFilePath), "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return fmt.Errorf("failed to load service account credentials: %w", err)
	}

	// Create HTTP client with the token
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add Authorization header with Bearer token
	token, err := creds.TokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to retrieve token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	// Perform the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download artifact, status code: %d", resp.StatusCode)
	}

	// Determine the file name from the Content-Disposition header or use a default name
	contentDisposition := resp.Header.Get("Content-Disposition")
	fileName := "tmp/downloaded-file"
	if contentDisposition != "" {
		_, params, err := mime.ParseMediaType(contentDisposition)
		if err == nil {
			if name, ok := params["filename"]; ok {
				fileName = name
			}
		}
	}

	fmt.Printf("Saving file as: %s\n", fileName)

	// Write the response to a file
	out, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// readFile reads the content of the service account key JSON file
func readFile(path string) []byte {
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("\U0001F534Error reading file %s: %v\U0001F534\n", path, err)
		os.Exit(1)
	}
	return content
}

func NewVersion(currentVersion, indexVersion, layout string) int {

	var newVersion int

	currentVersionParsed, err := time.Parse(layout, currentVersion)

	if err != nil {
		fmt.Printf("\U0001F534Error parsing version of the current version running: %v\U0001F534\n", err)
	}

	indexVersionParsed, err := time.Parse(layout, indexVersion)

	if err != nil {
		fmt.Printf("\U0001F534Error parsing the version that the index.json indicates: %v\U0001F534\n", err)
	}

	if currentVersionParsed.Before(indexVersionParsed) {
		newVersion = 1
	} else if currentVersionParsed.After(indexVersionParsed) {
		newVersion = 0
	} else {
		newVersion = 0
	}
	return newVersion
}

// Printing the expiratin date of a version

func PrintExpirationDate(layout, currentVersion string) {

	// Parse the string into a time.Time object
	currentVersionParsed, err := time.Parse(layout, currentVersion)

	if err != nil {
		fmt.Printf("\U0001F534Error parsing the current version date: %v\U0001F534\n", err)
		return
	}

	expirationDateOfCurrentVersion := currentVersionParsed.AddDate(2, 0, 0)

	currentDate := time.Now()

	validTimeOfCurrentVersion := expirationDateOfCurrentVersion.Sub(currentDate)

	totalHours := int(validTimeOfCurrentVersion.Hours())
	totalDays := totalHours / 24
	years := totalDays / 365
	days := totalDays % 365
	hours := totalHours % 24
	minutes := int(validTimeOfCurrentVersion.Minutes()) % 60
	seconds := int(validTimeOfCurrentVersion.Seconds()) % 60

	fmt.Printf("\u23F0The current version will expire in %d years, %d days, %d hours, %d minutes, and %d seconds\u23F0\n",
		years, days, hours, minutes, seconds)

}

func verifyingDownloadedFile(indexPath, DonwloadedFilePath string) int {

	// Hash of the index.json file
	var data map[string]indexInfo

	// Parse JSON into the map
	err := json.Unmarshal([]byte(indexPath), &data)
	if err != nil {
		fmt.Printf("\U0001F534Error parsing JSON: %v\U0001F534", err)
	}
	// Latest version considering the index.json downloaded by TUF

	indexHash := data["nebula-standalone"].Hashes.Sha256

	// Computing the hash of the downloaded file

	// Compute the SHA256 hash
	downloadedFilehash, err := ComputeSHA256(DonwloadedFilePath)
	if err != nil {
		fmt.Printf("\U0001F534Error computing hash: %v\U0001F534\n", err)
		return 0
	}

	if indexHash == downloadedFilehash {

		fmt.Printf("\U0001F7E2The target file has been downloaded successfully!\U0001F7E2\n")
		return 1
	} else {
		fmt.Printf("\U0001F534There has been an error while downloading the file\U0001F534\n")
		return 0
	}

}

func ComputeSHA256(filePath string) (string, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create a SHA256 hash object
	hasher := sha256.New()

	// Copy the file contents into the hasher
	// This reads the file in chunks to handle large files efficiently
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}

	// Get the final hash as a byte slice and convert to a hexadecimal string
	hash := hasher.Sum(nil)
	return fmt.Sprintf("%x", hash), nil
}
