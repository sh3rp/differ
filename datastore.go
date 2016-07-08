package differ

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/satori/go.uuid"
)

// data structures

type Tracked struct {
	Id         []byte
	Command    string
	Host       string
	Interval   int64 // seconds
	LastPolled int64
	InProgress bool
}

type Result struct {
	TrackedId []byte
	Timestamp time.Time
	Hash      []byte
	Result    []byte
}

const TRACKED_BUCKET = "tracked"
const RESULT_BUCKET = "result"

type TrackedStore interface {
	Track(string, string, int64) uuid.UUID
	Untrack(uuid.UUID) error
	SaveTracked(*Tracked) error
	GetTracked([]byte) *Tracked
	AllTracked() []*Tracked
	RecordState(*Result) error
	History(uuid.UUID) ([]*Result, error)
}

type BoltTrackedStore struct {
	DB *bolt.DB
}

func NewBoltTrackedStore() (*BoltTrackedStore, error) {
	db, err := bolt.Open("differ.db", 0600, &bolt.Options{Timeout: 1 * time.Second})

	if err != nil {
		return nil, err
	}

	return &BoltTrackedStore{
		DB: db,
	}, nil
}

func (ds *BoltTrackedStore) GetTracked(id []byte) *Tracked {
	db := ds.DB

	var tracked Tracked

	db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(TRACKED_BUCKET))
		if err != nil {
			log.Printf("GetTracked: %v", err)
		}
		v := b.Get(id)
		err = json.Unmarshal(v, &tracked)
		return err
	})

	return &tracked
}

func (ds *BoltTrackedStore) Track(command string, host string, interval int64) []byte {

	hash := sha256.New()
	hash.Write([]byte(command))
	hash.Write([]byte(host))

	tracked := &Tracked{
		Id:         hash.Sum(nil),
		Command:    command,
		Host:       host,
		Interval:   interval,
		LastPolled: 0,
	}

	db := ds.DB

	db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(TRACKED_BUCKET))
		json, err := json.Marshal(tracked)
		b.Put(tracked.Id, []byte(json))
		if err != nil {
			return fmt.Errorf("error creating bucket: %v", err)
		}

		return nil
	})

	return tracked.Id

}

func (ds *BoltTrackedStore) AllTracked() []*Tracked {
	db := ds.DB
	var tracked []*Tracked
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(TRACKED_BUCKET))
		cursor := b.Cursor()

		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var t Tracked
			json.Unmarshal(v, &t)
			tracked = append(tracked, &t)
		}
		return nil
	})

	return tracked
}

func (ds *BoltTrackedStore) Untrack(id uuid.UUID) error {
	return nil
}

func (ds *BoltTrackedStore) SaveTracked(tracked *Tracked) error {
	db := ds.DB

	db.Update(func(tx *bolt.Tx) error {
		b, _ := tx.CreateBucketIfNotExists([]byte(TRACKED_BUCKET))
		json, _ := json.Marshal(tracked)
		b.Put(tracked.Id, []byte(json))
		return nil
	})

	return nil
}

func (ds *BoltTrackedStore) RecordState(result *Result) error {
	db := ds.DB

	db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(RESULT_BUCKET))
		id := new(bytes.Buffer)
		err = binary.Write(id, binary.LittleEndian, result.TrackedId)
		err = binary.Write(id, binary.LittleEndian, "/")
		err = binary.Write(id, binary.LittleEndian, result.Timestamp.Unix())
		json, _ := json.Marshal(result)
		err = b.Put(id.Bytes(), []byte(json))
		return err
	})

	return nil
}

func (ds *BoltTrackedStore) History(id []byte) ([]*Result, error) {
	db := ds.DB

	var results []*Result

	db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(RESULT_BUCKET))
		cursor := b.Cursor()
		for k, v := cursor.Seek(id); bytes.HasPrefix(k, id); k, v = cursor.Next() {
			var result Result
			err = json.Unmarshal(v, &result)
			results = append(results, &result)
		}
		return err
	})

	return results, nil
}
