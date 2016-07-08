package differ

import (
	"crypto/sha256"
	"fmt"
	"log"
	"time"

	"github.com/sh3rp/sshexec"
)

const USERNAME = ""
const PASSWORD = ""

type Executor struct {
	Agent     *sshexec.SSHExecAgent
	Datastore *BoltTrackedStore
	running   bool
}

func NewExecutor() *Executor {
	agent := sshexec.NewAgent()
	agent.Start()
	ds, _ := NewBoltTrackedStore()

	executor := &Executor{
		Agent:     agent,
		Datastore: ds,
	}

	agent.AddListener(executor.HandleResult)

	return executor
}

func (exec *Executor) Track(host string, command string, interval int64) []byte {
	id := exec.Datastore.Track(command, host, interval)
	return id
}

func (exec *Executor) StartPoller() {
	exec.running = true
	log.Printf("Starting poller...\n")
	go func() {
		for exec.running {
			allTracked := exec.Datastore.AllTracked()

			for _, tracked := range allTracked {
				lp := tracked.LastPolled
				in := tracked.Interval
				if (lp+in) <= time.Now().Unix() && !tracked.InProgress {
					exec.Agent.RunWithCreds(USERNAME, PASSWORD, tracked.Host, 22, tracked.Command)
					tracked.InProgress = true
					exec.Datastore.SaveTracked(tracked)
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()
}

func (exec *Executor) StopPoller() {
	exec.running = false
}

func (exec *Executor) History(id []byte) []*Result {
	results, _ := exec.Datastore.History(id)
	return results
}

func (exec *Executor) HandleResult(result *sshexec.ExecResult) {

	if result == nil {
		return
	}

	hash := sha256.New()
	hash.Write([]byte(result.Command))
	hash.Write([]byte(result.Host))

	id := hash.Sum(nil)

	history, err := exec.Datastore.History(id)

	if err != nil {
		fmt.Println("Whoops")
	}

	hSize := len(history)

	hash.Reset()
	hash.Write(result.Result.Bytes())
	resultHash := hash.Sum(nil)

	if hSize == 0 || !eq(history[hSize-1].Hash, resultHash) {
		exec.Datastore.RecordState(&Result{
			TrackedId: id,
			Timestamp: result.EndTime,
			Hash:      resultHash,
			Result:    result.Result.Bytes(),
		})
	}

	tracked := exec.Datastore.GetTracked(id)
	tracked.LastPolled = time.Now().Unix()
	tracked.InProgress = false
	exec.Datastore.SaveTracked(tracked)
}

func eq(a []byte, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for idx := range a {
		if a[idx] != b[idx] {
			return false
		}
	}
	return true
}
