package webserver

import (
	"emlserver/comment"
	"emlserver/config"
	"emlserver/database"
	"emlserver/ffmpeg"
	"emlserver/helper"
	"emlserver/mail"
	"emlserver/message"
	"emlserver/mod"
	"emlserver/security"
	"emlserver/structs"
	"emlserver/ticket"
	"emlserver/tunnels"
	"emlserver/user"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	goaway "github.com/TwiN/go-away"
	"github.com/rs/cors"
)

func generateOTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if !query.Has("token") {
		http.Error(w, "does not have token as a query param", http.StatusBadRequest)
		return
	}
	row := database.Database.QueryRow("SELECT generated FROM otp WHERE token=$1", query["token"][0])
	var generated bool
	err := row.Scan(&generated)
	if err != nil || generated {
		if err != nil {
			println(err.Error())
		}
		w.Write([]byte("Unable to find a One-Time-Password with that token. ):"))
		return
	}
	otp := security.Hash(base64.StdEncoding.EncodeToString([]byte(security.GenerateUUID())))
	_, err = database.Database.Exec("UPDATE otp SET generated=1::bit, otp=$1 WHERE token=$2", security.Hash(otp), query["token"][0])
	if err != nil {
		println(err.Error())
		return
	}
	toSend := fmt.Sprintf("Your One-Time-Password is: %s\nPlease copy the password now as you will not be able to generate another one with this link without sending another request.", otp)
	w.Write([]byte(toSend))
}

func forgotPassword(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestRegisterAccount
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		return
	}

	emailHash := security.Hash(data.Email)

	row := database.Database.QueryRow("SELECT id FROM users WHERE emailhash=$1", emailHash)

	var id string

	err = row.Scan(&id)
	if err != nil {
		println(err.Error())
		return
	}

	println("Sending Forgot Password Email to ", id)

	token := security.GenerateUUID()

	_, err = database.Database.Exec("INSERT INTO otp (userid, expiredate, token, generated) VALUES ($1, $2, $3, 0::bit)", id, time.Now().Unix()+300, token)
	if err != nil {
		return
	}

	mail.ForgotPasswordEmail(data.Email, id, token)
}

func likePage(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestLike
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		return
	}

	user, err := user.GetUserWithToken(data.Token)
	if err != nil {
		return
	}

	res, err := database.LikePage(user, data.PageID)
	err = database.UpdateModCachedLikes(data.PageID)
	json.NewEncoder(w).Encode(res)
}

func isLiked(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestLike
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		return
	}

	res := structs.ResponseLiked{
		Liked: true,
	}

	user, err := user.GetUserWithToken(data.Token)

	if err != nil {
		res.Liked = false
	} else {
		err = database.UserLikedPage(user, data.PageID)
		if err != nil {
			res.Liked = false
		}
	}

	json.NewEncoder(w).Encode(res)
}

func setEmail(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestRegisterAccount
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		println(err.Error())
		return
	}

	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {
		println(data.Token)
		println(err.Error())
		return
	}

	encryptedEmail := security.Encrypt(data.Email)
	_, err = database.Database.Exec("UPDATE users SET email=$1, emailhash=$2 WHERE id=$3", encryptedEmail, security.Hash(data.Email), userID)
	if err != nil {
		println(err.Error())
		return
	}
}

func GetUserObj(Body io.ReadCloser, w http.ResponseWriter) (structs.User, error) {
	var data structs.RequestRegisterAccount
	err := json.NewDecoder(Body).Decode(&data)
	if err != nil {
		return structs.User{}, err
	}
	userObj, err := database.GetUser(data.ID)
	if err != nil {
		return structs.User{}, err
	}
	return userObj, nil
}

func getPing(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte(strconv.FormatInt(time.Now().UnixMilli(), 10)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func getUserPfp(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if !query.Has("id") {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	path, _ := user.GetProfilePicturePath(query["id"][0])
	http.ServeFile(w, r, path)
}

func getModIcon(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if !query.Has("id") {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	path := fmt.Sprint("static/modimg/", query["id"][0], ".webp")
	_, err := os.Open(path)
	if !errors.Is(err, os.ErrNotExist) {
		http.ServeFile(w, r, path)
	} else {
		http.ServeFile(w, r, "static/modimg/notfound.webp")
	}
}

func registerUser(w http.ResponseWriter, r *http.Request) {
	if config.LoadedConfig["ALLOW_REGISTRATION"] != "off" {
		http.Error(w, "sorry, we are not allowing registrations at the moment.", http.StatusForbidden)
		return
	}

	var data structs.RequestRegisterAccount
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	token, err := user.CreateUser(data.Username, data.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	_, err = w.Write([]byte(token))
	if err != nil {
		println("Error writing response for registering user account.")
		return
	}
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestToken
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, "failed to parse json body", http.StatusForbidden)
		return
	}

	userid, err := user.GetUserWithToken(data.Token)
	if err != nil {
		http.Error(w, "could not find user with specified token", http.StatusForbidden)
	}

	err = user.DeleteUser(userid)
	if err != nil {
		http.Error(w, "could not delete user, please contact administrator", http.StatusForbidden)
	}
}

func getUserBio(w http.ResponseWriter, r *http.Request) {
	userObj, err := GetUserObj(r.Body, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	_, err = w.Write([]byte(userObj.Bio))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
	}
}

func getUserEmail(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestToken
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, "failed to parse json body", http.StatusForbidden)
		return
	}

	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	userObj, err := database.GetUser(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if userObj.Email == "" {
		w.Write([]byte(""))
		return
	}

	decryptedEmail := security.Decrypt(userObj.Email)

	_, err = w.Write([]byte(decryptedEmail))
	if err != nil {
		return
	}
}

func getUsername(w http.ResponseWriter, r *http.Request) {
	userObj, err := GetUserObj(r.Body, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	_, err = w.Write([]byte(userObj.Username))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
	}
}

func getIDFromToken(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestRegisterAccount
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	_, err = w.Write([]byte(userID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
	}
}

func setUsername(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestRegisterAccount
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = user.ValidateUsername(data.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	database.SetUsername(userID, data.Username)
}

func setPassword(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestRegisterAccount
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = user.ValidatePassword(data.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = database.SetPassword(userID, data.Password)
	if err != nil {
		return
	}
}

func setBio(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestRegisterAccount
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	isProfane := goaway.IsProfane(data.Bio)
	if isProfane {
		http.Error(w, "please do not include profanities in your bio", http.StatusBadRequest)
		return
	}
	database.SetBio(userID, data.Bio)
}

func loginUser(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestRegisterAccount
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if data.Token != "" {
		_, err := user.GetUserWithToken(data.Token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		} else {
			w.WriteHeader(http.StatusOK)
			return
		}
	} else {
		token, err := user.LoginUser(data.Username, data.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		} else {
			_, err = w.Write([]byte(token))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			return
		}
	}
}

func setPfpUser(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		println(err.Error())
		http.Error(w, errors.New("upload error").Error(), http.StatusForbidden)
		return
	}
	token := r.MultipartForm.Value["token"]
	if token == nil {
		http.Error(w, errors.New("no token provided").Error(), http.StatusForbidden)
		return
	}

	userID, err := user.GetUserWithToken(token[0])

	println("Set pfp for: ", userID)

	if err != nil {
		http.Error(w, "failed to get user", http.StatusForbidden)
		return
	}

	err = helper.CreateTemp()
	if err != nil {
		println("couldn't create temp dir")
		return
	}
	defer helper.RemoveTemp()

	err = handleMultipartFile(r, "image", "tmp/icon", 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if !ffmpeg.ResizeImage("tmp/icon", 512, 512, fmt.Sprint("static/pfp/", userID, ".webp")) {
		http.Error(w, "failed to add pfp", http.StatusForbidden)
		return
	}
}

func getModArchive(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if !query.Has("id") {
		http.Error(w, "mod not found", http.StatusForbidden)
		return
	}
	path := fmt.Sprint("static/mods/", query["id"][0], ".tar.gz")
	http.ServeFile(w, r, path)
}

func publishMod(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestModUpload
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Token) == "" {
		http.Error(w, errors.New("no token provided").Error(), http.StatusForbidden)
		return
	}

	userid, err := user.GetUserWithToken(data.Token)
	if err != nil {
		http.Error(w, "failed to get user with provided token", http.StatusForbidden)
		return
	}

	tunnel, err := tunnels.CreateTunnel()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	mode := mod.DownloadAndCreate

	if data.ID != "" {
		modObj, err := database.GetMod(data.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		if modObj.Author != userid {
			http.Error(w, "you don't own this mod", http.StatusNotFound)
			return
		}

		mode = mod.UpdateAndPackage
	}

	go mod.HandleModRepository(tunnel, mode, data.ID, userid)
	println("tunnel created for mod upload:", tunnel)
	_, err = w.Write([]byte(tunnel))
	if err != nil {
		println("Error writing response for publishing mod")
		http.Error(w, "server error", http.StatusForbidden)
		return
	}
}

func queryMods(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestModQuery
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mods, rawCount, err := database.QueryMods(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	modObjs, err := mod.GetModsInBulk(mods)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := structs.ResponseModQuery{
		ModObjs:      modObjs,
		RawQuerySize: rawCount,
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func getModsByUser(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestRegisterAccount
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func getModPageCount(w http.ResponseWriter, r *http.Request) {
	row := database.Database.QueryRow("SELECT COUNT(*) FROM mods")
	var count int
	err := row.Scan(&count)
	if err != nil {
		http.Error(w, "failed to get mod page count", http.StatusForbidden)
	}
	pages := count / database.ModQueryLimit

	_, err = w.Write([]byte(strconv.Itoa(pages)))
	if err != nil {
		http.Error(w, "failed to write mod page count", http.StatusForbidden)
	}
}

func getModCount(w http.ResponseWriter, r *http.Request) {
	row := database.Database.QueryRow("SELECT COUNT(*) FROM mods")
	var count int
	err := row.Scan(&count)
	if err != nil {
		http.Error(w, "failed to get mod count", http.StatusForbidden)
	}
	_, err = w.Write([]byte(strconv.Itoa(count)))
	if err != nil {
		http.Error(w, "failed to write mod count", http.StatusForbidden)
	}
}

func handleMultipartFile(r *http.Request, formfile string, destination string, sizeLimitMB int64) error {
	file, handler, err := r.FormFile(formfile)
	if err != nil {
		return errors.New("failed to retrieve file")
	}
	if handler != nil {
		if handler.Size > sizeLimitMB*1024*1024 {
			return errors.New(fmt.Sprint("file too big (>", sizeLimitMB, "MB)"))
		}
	}
	defer file.Close()
	dst, err := os.Create(destination)
	if err != nil {
		println(err.Error())
		return errors.New("server error")
	}
	defer dst.Close()
	_, err = io.Copy(dst, file)
	if err != nil {
		println("Couldn't copy file to temporary file path.")
		return errors.New("server error")
	}
	return nil
}

func getMod(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestMod
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	modObj, err := database.GetMod(data.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = json.NewEncoder(w).Encode(modObj)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func getCommentCount(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestPageComments
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	count, err := database.GetCommentCount(data.PageID)
	if err != nil {
		_, err = w.Write([]byte("0"))
		if err != nil {
			return
		}

		return
	}

	_, err = w.Write([]byte(fmt.Sprint(count)))
}

func expiredRateLimits() {
	dur, err := time.ParseDuration("5s")
	if err != nil {
		panic(err)
	}
	for {
		time.Sleep(dur)
		res, err := database.Database.Exec("DELETE FROM ratelimits WHERE expirydate<$1", time.Now().Unix())
		if err != nil {
			println(err.Error())
		}

		affectedRows, err := res.RowsAffected()
		if err != nil {
			println(err.Error())
		}

		if affectedRows > 0 {
			println("Removed ", affectedRows, " expired ratelimit(s)")
		}
	}
}

func expiredOTPRoutine() {
	dur, err := time.ParseDuration("30s")
	if err != nil {
		panic(err)
	}
	for {
		time.Sleep(dur)
		res, err := database.Database.Exec("DELETE FROM otp WHERE expiredate<$1", time.Now().Unix())
		if err != nil {
			println(err.Error())
		}

		affectedRows, err := res.RowsAffected()
		if err != nil {
			println(err.Error())
		}

		if affectedRows > 0 {
			println("Removed ", affectedRows, " expired otp ticket(s)")
		}
	}
}

func deleteMod(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestModUpload
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	modObj, err := database.GetMod(data.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if modObj.Author != userID {
		http.Error(w, "you do not own this mod", http.StatusForbidden)
	}

	mod.DeleteMod(modObj.ID)
}

func sendReport(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestReport
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ticket.AddTicket("Report of "+data.TargetID+" Reason: "+data.ReportReason, "report", data.TargetID, "", userID)
}

func getMessageCount(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestToken
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var count int
	row := database.Database.QueryRow("SELECT COUNT(*) FROM messages WHERE toid=$1", userID)
	err = row.Scan(&count)
	if err != nil {

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write([]byte(fmt.Sprint(count)))
}

func getMessages(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestToken
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	msgs, err := message.GetMessagesForUser(userID)
	if err != nil {

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = json.NewEncoder(w).Encode(msgs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func deleteMessage(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestMessage
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	msg, err := message.GetMessage(data.ID)
	if err != nil {

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if msg.To != userID {
		http.Error(w, "you are not the recipient of this message", http.StatusBadRequest)
	}

	err = message.DeleteMessage(data.ID)
	if err != nil {

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func updateEmailOptions(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestEmailOptions
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	parsedMessages, err := strconv.Atoi(data.Messages)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = mail.UpdateEmailOptions(userID, parsedMessages)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func deleteComment(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestComment
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	commentObj, err := comment.GetComment(data.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if commentObj.Author != userID {

		http.Error(w, "you are not the author of this comment", http.StatusBadRequest)
		return
	}

	err = comment.DeleteComment(data.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func sendComment(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestComment
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	userID, err := user.GetUserWithToken(data.Token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(data.Content) == "" {
		http.Error(w, "comment is empty", http.StatusBadRequest)
		return
	}
	err = comment.SendComment(userID, data.PageID, data.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func getComment(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestPageComments
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	commentObj, err := comment.GetComment(data.PageID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(commentObj)
}

func queryComment(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestPageComments
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	commentQuery, err := comment.QueryComments(data.PageID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(commentQuery)
}

func userCount(w http.ResponseWriter, r *http.Request) {
	row := database.Database.QueryRow("SELECT COUNT(*) FROM users")
	var count int
	row.Scan(&count)
	w.Write([]byte(fmt.Sprint(count)))
}

func incrementModDownloads(w http.ResponseWriter, r *http.Request) {
	var data structs.RequestMod
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	addr := getIP(r)
	if addr == "" {
		http.Error(w, "couldn't retrieve IP address for ratelimit.", http.StatusBadRequest)
		return
	}
	hashedAddr := security.Hash(addr)

	if database.HasRateLimit(addr, "mod_increment_downloads", data.ID) {
		return
	}

	expiryDate, err := time.ParseDuration("24h")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mod, err := database.GetMod(data.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = database.AddRateLimit(hashedAddr, time.Now().Unix()+int64(expiryDate.Seconds()), "mod_increment_downloads", data.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = database.Database.Exec("UPDATE mods SET downloads=$1 WHERE id=$2", mod.Downloads+1, data.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func getIP(r *http.Request) string {
	addr := r.Header.Get("X-Real-Ip")
	if addr == "" {
		addr = r.Header.Get("X-Forwarded-For")
	}
	if addr == "" {
		addr = r.RemoteAddr
	}
	return addr
}

func DownloadTool(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if !query.Has("target") {
		println("no target")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if !query.Has("tool") {
		println("no tool")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	var file = ""
	if query.Get("tool") == "dolphin" {
		if query.Get("target") == "linux" {
			file = "static/dolphin/dolphin_linux.tar.gz"
		}
		if query.Get("target") == "windows" {
			file = "static/dolphin/dolphin_windows.tar.gz"
		}
		if query.Get("target") == "macos" {
			file = "static/dolphin/dolphin_macos.zip"
		}
	}

	if query.Get("tool") == "ue4ss" {
		file = "static/ue4ss/ue4ss.tar.gz"
	}

	if file == "" {
		http.Error(w, "tool target could not be found", http.StatusBadRequest)
		return
	}
	http.ServeFile(w, r, file)

}

func InitTunnel(w http.ResponseWriter, r *http.Request) {
	var data structs.TunnelInitData
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = tunnels.InitTunnel(data.TunnelID, data.ChunkSize, data.Chunks, data.FileSize)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		print(err.Error())
		return
	}
}

func AddChunkFromTunnel(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		println(err.Error())
		http.Error(w, errors.New("upload error").Error(), http.StatusForbidden)
		return
	}

	if r.MultipartForm.Value["tunnelid"] == nil {
		println(err.Error())
		http.Error(w, errors.New("no tunnel id provided").Error(), http.StatusForbidden)
		return
	}
	tunnelid := r.MultipartForm.Value["tunnelid"][0]
	err = tunnels.ProcessTunnelChunk(tunnelid, r)
	if err != nil {
		http.Error(w, errors.New("no tunnel id provided").Error(), http.StatusForbidden)
		print(err.Error())
	}
}

func InitializeWebserver() {
	go expiredOTPRoutine()
	go expiredRateLimits()

	mux := http.NewServeMux()

	// bindings
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("Epic Mickey Launcher Server"))
		if err != nil {
			return
		}
	})

	// start image server

	mux.HandleFunc("/img/userpfp", getUserPfp)
	mux.HandleFunc("/img/modicon", getModIcon)

	// end image server

	// start user

	mux.HandleFunc("/user/username", getUsername)
	mux.HandleFunc("/user/email", getUserEmail)
	mux.HandleFunc("/user/bio", getUserBio)
	mux.HandleFunc("/user/idfromtoken", getIDFromToken)
	mux.HandleFunc("/user/register", registerUser)
	mux.HandleFunc("/user/login", loginUser)
	mux.HandleFunc("/user/report", sendReport)
	mux.HandleFunc("/user/messages", getMessages)
	mux.HandleFunc("/user/messagecount", getMessageCount)
	mux.HandleFunc("/user/deletemessage", deleteMessage)
	mux.HandleFunc("/user/delete", deleteUser)
	mux.HandleFunc("/user/set/pfp", setPfpUser)
	mux.HandleFunc("/user/set/username", setUsername)
	mux.HandleFunc("/user/set/password", setPassword)
	mux.HandleFunc("/user/set/bio", setBio)
	mux.HandleFunc("/user/set/email", setEmail)
	mux.HandleFunc("/user/set/email/options", updateEmailOptions)
	mux.HandleFunc("/user/otp", forgotPassword)
	mux.HandleFunc("/user/otp/auth", generateOTP)
	mux.HandleFunc("/user/count", userCount)
	// end user

	// start mod

	mux.HandleFunc("/mod/query", queryMods)
	mux.HandleFunc("/mod/get", getMod)
	mux.HandleFunc("/mod/pagecount", getModPageCount)
	mux.HandleFunc("/mod/count", getModCount)
	mux.HandleFunc("/mod/publish", publishMod)
	mux.HandleFunc("/mod/delete", deleteMod)
	mux.HandleFunc("/mod/download", getModArchive)
	mux.HandleFunc("/mod/download/increment", incrementModDownloads)

	// end mod

	mux.HandleFunc("/server/ping", getPing)

	// start comment
	mux.HandleFunc("/comment/send", sendComment)
	mux.HandleFunc("/comment/delete", deleteComment)
	mux.HandleFunc("/comment/query", queryComment)
	mux.HandleFunc("/comment/get", getComment)
	mux.HandleFunc("/comment/count", getCommentCount)
	// end comment

	// start like
	mux.HandleFunc("/like/add", likePage)
	mux.HandleFunc("/like/liked", isLiked)
	// end like

	// start tool
	mux.HandleFunc("/tool/download", DownloadTool)
	// end tool

	//start tunnel
	mux.HandleFunc("/tunnel/init", InitTunnel)
	mux.HandleFunc("/tunnel/chunk", AddChunkFromTunnel)
	//end tunnel

	// end bindings

	handler := cors.Default().Handler(mux)

	err := http.ListenAndServe(":8574", handler)
	if err != nil {
		panic(err)
	}
}
