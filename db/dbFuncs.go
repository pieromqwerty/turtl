package db

import (
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"math/rand"
	"strings"
	"time"
	"turtl/structs"
	"turtl/utils"
)

func IsFileBlacklisted(sha256 string) (bool, bool) {
	blacklist, err := DB.Query("select * from blacklist where hash=$1", strings.ToUpper(sha256))
	if utils.HandleError(err, "check if file is blacklisted") {
		return true, false
	}
	defer blacklist.Close()
	if blacklist.Next() {
		return true, true
	}
	return false, true
}

func DoesFileSumExist(md5 string, sha256 string, domain string) (string, bool) {
	objects, err := DB.Query("select * from objects where (md5=$1 or sha256=$2) and bucket=$3", strings.ToUpper(md5), strings.ToUpper(sha256), domain)
	if utils.HandleError(err, "check if file sum exists") {
		return "", false
	}
	defer objects.Close()
	if objects.Next() {
		var existingObject structs.Object
		err = objects.Scan(&existingObject.Bucket, &existingObject.Wildcard, &existingObject.FileName, &existingObject.Uploader, &existingObject.CreatedAt, &existingObject.MD5, &existingObject.SHA256, &existingObject.DeletedAt)
		if existingObject.Wildcard == "" {
			return "https://" + existingObject.Bucket + "/" + existingObject.FileName, true
		} else {
			return "https://" + existingObject.Wildcard + "." + existingObject.Bucket + "/" + existingObject.FileName, true
		}
	}
	return "", true
}

func GetFileFromURL(url string) (structs.Object, bool) {
	if url == "" {
		return structs.Object{}, false
	}
	if strings.Count(url, ".") < 2 {
		return structs.Object{}, false
	}

	url = strings.TrimPrefix(url, "https://")
	splitAtPeriods := strings.Split(url, ".")
	splitAtSlash := strings.Split(url, "/")
	filename := splitAtSlash[1]

	var wildcard string
	var domain string
	if len(splitAtPeriods) == 3 { // no wildcard
		domain = splitAtPeriods[0] + "." + strings.TrimSuffix(splitAtPeriods[1], "/"+strings.Split(filename, ".")[0])
		wildcard = ""
	} else {
		domain = splitAtPeriods[1] + "." + strings.TrimSuffix(splitAtPeriods[2], "/"+strings.Split(filename, ".")[0])
		wildcard = splitAtPeriods[0]
	}

	rows, err := DB.Query("select * from objects where wildcard=$1 and bucket=$2 and filename=$3", wildcard, domain, filename)
	if utils.HandleError(err, "query DB for GetFileFromURL") {
		return structs.Object{}, false
	}
	defer rows.Close()
	if rows.Next() {
		var retVal structs.Object
		err = rows.Scan(&retVal.Bucket, &retVal.Wildcard, &retVal.FileName, &retVal.Uploader, &retVal.CreatedAt, &retVal.MD5, &retVal.SHA256, &retVal.DeletedAt)
		if utils.HandleError(err, "scan into retval at GetFileFromURL") {
			return structs.Object{}, false
		}
		return retVal, true
	}
	return structs.Object{}, true
}

func GetBlacklist(sha256 string) (structs.Blacklist, bool) {
	rows, err := DB.Query("select * from blacklist where hash=$1", strings.ToUpper(sha256))
	if utils.HandleError(err, "check blacklist") {
		return structs.Blacklist{}, false
	}
	defer rows.Close()
	if rows.Next() {
		var retVal structs.Blacklist
		err = rows.Scan(&retVal.SHA256, &retVal.Reason)
		if utils.HandleError(err, "scan blacklist info into retval") {
			return structs.Blacklist{}, false
		}
		return retVal, true
	}
	return structs.Blacklist{}, true
}

func DoesFileNameExist(name string, domain string) (bool, bool) {
	rows, err := DB.Query("select * from objects where filename=$1 and bucket=$2", name, domain)
	if utils.HandleError(err, "query psql for file") {
		return true, false
	}
	defer rows.Close()
	if rows.Next() {
		return true, true
	}
	return false, true
}

func GenerateNewFileName(extension string, domain string) (string, bool) {
	characters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for a := 0; a < 5; a++ {
		rand.Seed(time.Now().UnixNano())
		b := make([]byte, 10)
		for i := range b {
			b[i] = characters[rand.Intn(len(characters))]
		}

		formatted := string(b) + "." + extension
		exists, ok := DoesFileNameExist(formatted, domain)
		if !ok {
			return "", false
		}
		if !exists {
			return formatted, true
		}
	}
	return "", false
}

func CheckAdmin(m *discordgo.Message) (bool, bool) {
	users, err := DB.Query("select * from users where discordid=$1 and admin=true", m.Author.ID)
	if utils.HandleError(err, "query users to check admin") {
		return false, false
	}
	defer users.Close()
	if !users.Next() {
		return false, true
	}
	return true, true
}

func CreateUser(member *discordgo.Member) (string, bool) {
	generated, ok := GenerateUUID()
	if !ok || generated == "" {
		return "", false
	}

	_, err := DB.Exec("insert into users values ($1, $2, 50000000, false)", member.User.ID, generated)
	if utils.HandleError(err, "query users to check for existing uuid") {
		return "", false
	}

	return generated, true
}

func GenerateUUID() (string, bool) {
	var generated uuid.UUID
	for i := 0; i < 5; i++ {
		generated = uuid.New()
		exists, ok := DoesUserExist(generated.String())
		if !ok {
			return "", false
		}
		if !exists {
			return generated.String(), true
		}
	}
	return "", false
}

func RevokeKey(key string) bool {
	_, err := DB.Exec("delete from users where discordid=$1 or apikey=$1", key)
	if utils.HandleError(err, "delete user from db") {
		return false
	}
	return true
}

func DoesUserExist(apikey string) (bool, bool) {
	existing, err := DB.Query("select * from users where apikey=$1", apikey)
	if utils.HandleError(err, "query users to check for existing uuid") {
		return false, false
	}
	defer existing.Close()
	if existing.Next() {
		return true, true
	}

	return false, true
}

func GetAccountFromDiscord(userID string) (structs.User, bool) {
	users, err := DB.Query("select * from users where discordid=$1", userID)
	if utils.HandleError(err, "checking for discord account") {
		return structs.User{}, false
	}
	defer users.Close()
	if users.Next() {
		var retUser structs.User
		err = users.Scan(&retUser.DiscordID, &retUser.APIKey, &retUser.UploadLimit, &retUser.Admin)
		if utils.HandleError(err, "get discord member account scan") {
			return structs.User{}, false
		}
		return retUser, true
	}
	return structs.User{}, true
}

func GetAccountFromAPIKey(apikey string) (structs.User, bool) {
	users, err := DB.Query("select * from users where apikey=$1", apikey)
	if utils.HandleError(err, "checking for turtl account from apikey") {
		return structs.User{}, false
	}
	defer users.Close()
	if users.Next() {
		var retUser structs.User
		err = users.Scan(&retUser.DiscordID, &retUser.APIKey, &retUser.UploadLimit, &retUser.Admin)
		if utils.HandleError(err, "api key account scan") {
			return structs.User{}, false
		}
		return retUser, true
	}
	return structs.User{}, true
}
