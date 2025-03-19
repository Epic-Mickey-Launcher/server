package tunnels

import (
	"emlserver/database"
	"emlserver/security"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

func InitTunnel(tunnelid string, chunksize int, chunks int, filesize int) error {
	row := database.Database.QueryRow("SELECT (input) FROM tunnels WHERE tunnelid=$1", tunnelid)
	input := false
	err := row.Scan(&input)

	if err != nil {
		return err
	}

	if !input {
		return errors.New("tunnel is already killed")
	}

	if chunksize > 20*1024*1024 {
		return errors.New("client request for chunk size is too large")
	}

	if filesize > 3.5*1024*1024*1024 {
		return errors.New("client request file size is too large")
	}

	_, err = database.Database.Exec("UPDATE tunnels SET filesize=$1, chunksize=$2, chunksreceived=$3, chunks=$4 WHERE tunnelid=$5",
		filesize, chunksize, 0, chunks, tunnelid)
	if err != nil {
		return err
	}

	println("Tunnel Initialized\nTunnel ID: ", tunnelid, "\nChunk Size (b): ", chunksize, "\nChunks: ", chunks, "\nFile Size (b): ", filesize)

	return nil
}

func CreateTunnel() (string, error) {
	id := security.GenerateUUID()
	_, err := database.Database.Exec("INSERT INTO tunnels(tunnelid, chunksize, chunksreceived, chunks, input, filesize) VALUES ($1,0,0,0,true,0)", id)
	if err != nil {
		return "", err
	}
	return id, err
}

func RemoveTunnel(tunnelid string) error {
	_, err := database.Database.Exec("DELETE FROM tunnels WHERE tunnelid=$1", tunnelid)
	return err
}

func CheckTunnel(tunnelid string) (bool, error) {
	row := database.Database.QueryRow("SELECT (input) FROM tunnels WHERE tunnelid=$1", tunnelid)
	var input bool
	err := row.Scan(&input)

	return !input, err // no input allowed means that tunnel has received all chunks and the file is whole.
}

func GetTunnelFile(tunnelid string) (string, error) {
	path := "tunnelfiles/" + tunnelid
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	file.Close()

	return path, nil
}

func ProcessTunnelChunk(tunnelid string, r *http.Request) error {
	var (
		chunks         int
		chunksreceived int
		input          bool
	)

	row := database.Database.QueryRow("SELECT chunks, chunksreceived, input FROM tunnels WHERE tunnelid=$1", tunnelid)
	err := row.Scan(&chunks, &chunksreceived, &input)

	if err != nil {
		return err
	}

	if !input {
		return errors.New("tunnel already killed")
	}

	err = addChunkFromTunnelMultipart(r, fmt.Sprint("tunnelfiles/", tunnelid), 20)
	if err != nil {
		return err
	}

	chunksreceived += 1

	_, err = database.Database.Exec("UPDATE tunnels SET chunksreceived=$2 WHERE tunnelid=$1", tunnelid, chunksreceived)
	if err != nil {
		return err
	}
	println("Received Chunk")
	if chunksreceived == chunks {
		println("Tunnel Upload Complete... Killing Tunnel Input...")

		_, err = database.Database.Exec("UPDATE tunnels SET input=$2 WHERE tunnelid=$1", tunnelid, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func addChunkFromTunnelMultipart(r *http.Request, destination string, sizeLimitMB int64) error {
	err := os.MkdirAll("tunnelfiles/", os.ModePerm)
	if err != nil {
		return err
	}

	file, handler, err := r.FormFile("chunk")
	if err != nil {
		return errors.New("failed to retrieve file")
	}
	if handler != nil {
		if handler.Size > sizeLimitMB*1024*1024 {
			return errors.New(fmt.Sprint("chunk too big (>", sizeLimitMB, "MB)"))
		}
	}
	defer file.Close()

	dst, err := os.OpenFile(destination, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
