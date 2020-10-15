package tessera

import (
	"encoding/json"
	"net/http"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/private/engine"
)

const apiVersion1 = "1.0"

var knownApiVersions = map[string]bool{
	"1.0": true,
	"2.0": true,
}

func APIVersions(client *engine.Client) []string {
	res, err := client.Get("/version/api")
	if err != nil {
		log.Error("Error invoking the tessera /version/api API: %v.", err)
		return []string{}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Error("Invalid status code returned by the tessera /version/api API: %d.", res.StatusCode)
		return []string{}
	}
	var versions []string
	if err := json.NewDecoder(res.Body).Decode(&versions); err != nil {
		log.Error("Unable to deserialize the tessera response for /version/api API: %v.", err)
		return []string{}
	}
	if len(versions) == 0 {
		log.Error("Expecting at least one API version to be returned by the tessera /version/api API.")
		return []string{}
	}
	return versions
}

// this method will be removed once quorum will implement a versioned tessera client (in line with tessera API versioning)
func RetrieveTesseraAPIVersion(client *engine.Client) string {
	allRetrievedVersions := APIVersions(client)
	onlyKnownVersions := filterUnknownVersions(allRetrievedVersions)

	// pick the latest version from the versions array
	latestVersion := apiVersion1
	latestParsedVersion, _ := parseVersion([]byte(latestVersion))
	for _, ver := range onlyKnownVersions {
		if len(ver) == 0 {
			log.Error("Invalid (empty) version returned by the tessera /version/api API. Skipping value.")
			continue
		}
		parsedVer, err := parseVersion([]byte(ver))
		if err != nil {
			log.Error("Unable to parse version returned by the tessera /version/api API: %s. Skipping value.", ver)
			continue
		}
		if compareVersions(parsedVer, latestParsedVersion) > 0 {
			latestVersion = ver
			latestParsedVersion = parsedVer
		}
	}
	log.Info("Tessera API version: %s", latestVersion)
	return latestVersion
}

func filterUnknownVersions(retrievedVersions []string) []string {
	filtered := make([]string, 0)
	for _, version := range retrievedVersions {
		if _, ok := knownApiVersions[version]; ok {
			filtered = append(filtered, version)
		}
	}
	return filtered
}
