package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

const (
	MINING_DIFFICULTY              = 4
	MINING_REWARD                  = 10.0
	MINING_SENDER                  = "THE BLOCKCHAIN"
	BLOCK_GENERATION_INTERVAL      = 10
	DIFFICULTY_ADJUSTMENT_INTERVAL = 10
)

type Block struct {
	Index        int
	Timestamp    string
	BPM          int
	Hash         string
	PrevHash     string
	Transactions []Transactions
	Nonce        int
	Difficulty   int
}

type Transactions struct {
	From      string
	To        string
	Amount    float64
	TimeStamp string
}

var Blockchain []Block

func calculateHash(block Block) string {
	record := strconv.Itoa(block.Index) +
		block.Timestamp +
		strconv.Itoa(block.BPM) +
		block.PrevHash +
		strconv.Itoa(block.Nonce)
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func generateBlock(oldBlock Block, BPM int) (Block, error) {
	var newBlock Block

	t := time.Now()
	newBlock = Block{
		Index:      oldBlock.Index + 1,
		Timestamp:  t.String(),
		BPM:        BPM,
		PrevHash:   oldBlock.Hash,
		Difficulty: getNewDifficulty(oldBlock),
		Transactions: []Transactions{
			{
				From:      MINING_SENDER,
				To:        "MinerAddress", // This should be replaced with actual miner's address
				Amount:    MINING_REWARD,
				TimeStamp: t.String(),
			},
		},
	}

	mineBlock(&newBlock)
	return newBlock, nil
}

func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}
	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}
	if calculateHash(newBlock) != newBlock.Hash {
		return false
	}
	target := strings.Repeat("0", newBlock.Difficulty)
	if !strings.HasPrefix(newBlock.Hash, target) {
		return false
	}
	return true
}

func replaceChain(newBlocks []Block) {
	if len(newBlocks) > len(Blockchain) {
		Blockchain = newBlocks
	}
}

func makeMuxRouter() http.Handler {
	muxRouter := mux.NewRouter()
	muxRouter.HandleFunc("/", handleGetBlockchain).Methods("GET")
	muxRouter.HandleFunc("/", handleWriteBlock).Methods("POST")
	return muxRouter
}

func handleGetBlockchain(w http.ResponseWriter, r *http.Request) {
	bytes, err := json.MarshalIndent(Blockchain, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, string(bytes))
}

type Message struct {
	BPM int
}

func handleWriteBlock(w http.ResponseWriter, r *http.Request) {
	var m Message

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&m); err != nil {
		respondWithJSON(w, r, http.StatusBadRequest, r.Body)
		return
	}
	defer r.Body.Close()

	newBlock, err := generateBlock(Blockchain[len(Blockchain)-1], m.BPM)
	if err != nil {
		respondWithJSON(w, r, http.StatusInternalServerError, m)
		return
	}
	if isBlockValid(newBlock, Blockchain[len(Blockchain)-1]) {
		newBlockchain := append(Blockchain, newBlock)
		replaceChain(newBlockchain)
		spew.Dump(Blockchain)
	}

	respondWithJSON(w, r, http.StatusCreated, newBlock)

}

func respondWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
	response, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("HTTP 500: Internal Server Error"))
		return
	}
	w.WriteHeader(code)
	w.Write(response)
}

func mineBlock(block *Block) {
	target := strings.Repeat("0", block.Difficulty)

	for {
		block.Hash = calculateHash(*block)
		if strings.HasPrefix(block.Hash, target) {
			return
		}
		block.Nonce++
	}
}

func getNewDifficulty(lastBlock Block) int {
	if lastBlock.Index%DIFFICULTY_ADJUSTMENT_INTERVAL == 0 && lastBlock.Index != 0 {
		return adjustDifficulty(lastBlock)
	}
	return lastBlock.Difficulty
}

func adjustDifficulty(lastBlock Block) int {
	prevAdjustmentBlock := Blockchain[len(Blockchain)-DIFFICULTY_ADJUSTMENT_INTERVAL]

	lastTime, err := parseTimestamp(lastBlock.Timestamp)
	if err != nil {
		return lastBlock.Difficulty
	}

	prevTime, err := parseTimestamp(prevAdjustmentBlock.Timestamp)
	if err != nil {
		return lastBlock.Difficulty
	}

	expectedTime := int64(BLOCK_GENERATION_INTERVAL * DIFFICULTY_ADJUSTMENT_INTERVAL)
	timeTaken := lastTime - prevTime

	if timeTaken < expectedTime/2 {
		return prevAdjustmentBlock.Difficulty + 1
	} else if timeTaken > expectedTime*2 {
		return prevAdjustmentBlock.Difficulty - 1
	}
	return prevAdjustmentBlock.Difficulty
}

func parseTimestamp(timestamp string) (int64, error) {
	t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", timestamp)
	if err != nil {
		return 0, err
	}
	return t.Unix(), nil
}

func run() error {
	mux := makeMuxRouter()
	httpAddr := os.Getenv("ADDR")
	log.Println("Listening on ", os.Getenv("ADDR"))
	s := &http.Server{
		Addr:           ":" + httpAddr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	if err := s.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		t := time.Now()
		genesisBlock := Block{
			Index:        0,
			Timestamp:    t.String(),
			BPM:          0,
			Hash:         calculateHash(Block{}),
			PrevHash:     "",
			Transactions: []Transactions{},
			Nonce:        0,
			Difficulty:   0,
		}
		spew.Dump(genesisBlock)
		Blockchain = append(Blockchain, genesisBlock)
	}()
	log.Fatal(run())
}
