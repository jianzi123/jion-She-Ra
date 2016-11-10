package utils

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/golang/protobuf/proto"
	"github.com/haveatry/She-Ra/configdata"
	"github.com/magiconair/properties"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/websocket"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"runtime"
)

const (
	EXECUTION_PATH    = "executions/"
	MAX_EXEC_NUM      = 100
	MAX_KEEP_DAYS     = 3
	EXEC_GOROUTINE    = 1314
	EXEC_FINISHED     = 1
	EXEC_KILL_ALL     = 886
	EXEC_KILL_FAILURE = 8
	EXEC_ERROR        = 16
)

var Database *sql.DB
var WS_PATH string

type Key struct {
	Ns string
	Id string
}

type Execution struct {
	Namespace string
	JobId     string
	SeqNo     int32
	Progress  int32
	EndStatus int32
	Finished  int32
	Cancelled int32
	StartTime int64
	EndTime   int64
}

type ExecView struct {
	SeqNo     int32
	EndStatus int32
	EndTime   int64
}

type JobView struct {
	Status          int32
	JobId           string
	LastSuccessTime int64
	LastFailureTime int64
	LastDuration    int64
}

type JDK struct {
	Version string
	Path    string
}

type JobSock struct {
	NameSpace string `json:"namespace"`
	JobId     string `json:"jobid"`
	SeqNo     string `json:"seqno"`
	Flag      bool   `json:"flag"`
}

const (
	EXEC_INIT           int32 = 0
	EXEC_CODE_PULLING   int32 = 1
	EXEC_CODE_BUILDING  int32 = 2
	EXEC_IMAGE_BUILDING int32 = 3
	EXEC_IMAGE_PUSHING  int32 = 4
)

const (
	EXEC_SUCCESS int32 = 0
	EXEC_FAILURE int32 = 1
	EXEC_NONE    int32 = 2
)

const (
	EXEC_NOT_DONE int32 = 0
	EXEC_DONE     int32 = 1
)

const (
	EXEC_NOT_CANCELLED int32 = 0
	EXEC_CANCELLED     int32 = 1
)

func Init(props *properties.Properties) {
	//create database for She-Ra project
	var err error
	WS_PATH = props.MustGet("working.path")
	if err = os.MkdirAll(WS_PATH, 0770); err != nil {
		Info("failed to create workspace dir\n")
		os.Exit(1)

	}

	dbPath := props.MustGet("database.path")
	if err = os.MkdirAll(dbPath, 0770); err != nil {
		Info("failed to create database dir\n")
		os.Exit(1)

	}

	dbFile := dbPath + "She-Ra.db"
	if Database, err = sql.Open("sqlite3", dbFile); err != nil {
		Info("failed to setup database")
		os.Exit(1)
	}

	//create table job to store execution information
	sql := `Create table IF NOT EXISTS job(namespace CHAR(100) NOT NULL, jobId CHAR(100) NOT NULL,seqno integer NOT NULL,progress integer,status integer, finished integer, cancelled integer, startTime integer, endTime integer, PRIMARY KEY(namespace, jobId, seqno))`
	if _, err = Database.Exec(sql); err != nil {
		Info("failed to create table job")
		os.Exit(1)
	}

	//create job display info table
	sql = `Create table IF NOT EXISTS jobView(namespace CHAR(100) NOT NULL, jobId CHAR(100) NOT NULL,status integer, startTime integer, endTime integer, lastSuccessTime integer, lastFailureTime integer, PRIMARY KEY(namespace, jobId))`
	if _, err = Database.Exec(sql); err != nil {
		Info("failed to create table jobView")
		os.Exit(1)
	}

	//create table jdk to store execution information
	sql = `create table if not exists jdk(version, installpath string, PRIMARY KEY(version))`
	if _, err = Database.Exec(sql); err != nil {
		Info("failed to create table jdk")
		os.Exit(1)
	}
}

func InsertJobViewRecord(namespace, jobId string) error {
	if stmt, err := Database.Prepare("insert into jobView(namespace, jobId, status, startTime, endTime, lastSuccessTime, lastFailureTime) values (?,?,?,?,?,?,?)"); err != nil {
		Info("failed to prepare insert sql:%v\n", err)
		return err

	} else if _, err := stmt.Exec(namespace, jobId, EXEC_NONE, 0, 0, 0, 0); err != nil {
		Info("failed to insert data into database:%v\n", err)
		return err
	}

	return nil
}

func UpdateJobViewStartTime(namespace, jobId string, startTime int64) {
	if stmt, err := Database.Prepare("update jobView set startTime = ? where namespace = ? and jobId = ?"); err != nil {
		Info("failed to prepare update sql:%v\n", err)

	} else if _, err := stmt.Exec(startTime, namespace, jobId); err != nil {
		Info("failed to update data startTime:%v\n", err)
	}

}

func UpdateJobViewEndTime(namespace, jobId string, endTime int64) {
	if stmt, err := Database.Prepare("update jobView set endTime = ? where namespace = ? and jobId = ?"); err != nil {
		Info("failed to prepare update sql:%v\n", err)

	} else if _, err := stmt.Exec(endTime, namespace, jobId); err != nil {
		Info("failed to update data startTime:%v\n", err)
	}

}

func UpdateJobViewStatus(namespace, jobId string, endTime int64, status int32) {
	var sql string
	if status == EXEC_FAILURE {
		sql = "update jobView set endTime = ?, lastFailureTime = ?, status = ? where namespace = ? and jobId = ?"
	} else if status == EXEC_SUCCESS {
		sql = "update jobView set endTime = ?, lastSuccessTime = ?, status = ? where namespace = ? and jobId = ?"
	} else {
		Info("Not allowed to update jobView status when the status is invalid\n")
		return
	}

	if stmt, err := Database.Prepare(sql); err != nil {
		Info("failed to prepare update sql:%v\n", err)

	} else if _, err := stmt.Exec(endTime, endTime, status, namespace, jobId); err != nil {
		Info("failed to update data startTime:%v\n", err)
	}

}

func GetJobViewRecords(namespace string, jobView *[]JobView) error {

	var view JobView
	var startTime, endTime int64

	rows, err := Database.Query("select status, jobId, startTime, endTime, lastSuccessTime, lastFailureTime  from jobView where namespace =?", namespace)
	if err != nil {
		Info("failed to prepare query sql:%v\n", err)
		return err
	}

	for rows.Next() {
		if err := rows.Scan(&view.Status, &view.JobId, &startTime, &endTime, &view.LastSuccessTime, &view.LastFailureTime); err != nil {
			log.Println(err)
		}

		Info("get one jobView record\n")
		view.LastDuration = endTime - startTime
		Info("get one jobView record %v\n", view)
		*jobView = append(*jobView, view)
	}
	return nil
}

func InsertJdk(version, installPath string) {
	if stmt, err := Database.Prepare("insert into jdk(version, installPath) values (?,?)"); err != nil {
		Info("failed to prepare insert sql:%v\n", err)

	} else if _, err := stmt.Exec(version, installPath); err != nil {
		Info("failed to insert data into database:%v\n", err)
	}

}

func DeleteJdk(version string) error {
	if stmt, err := Database.Prepare("delete from jdk  where version = ?"); err != nil {
		Info("failed to prepare delete sql:%v\n", err)
		return err

	} else if _, err := stmt.Exec(version); err != nil {
		Info("failed to delete data from database:%v\n", err)
		return err
	}
	return nil
}

// delete a job execution record
func DelJobExecRecord(namespace, jobId string, seqno int) error {

	if stmt, err := Database.Prepare("delete from job where namespace = ? and jobId = ? and seqno = ? and finished = ?"); err != nil {
		Info("failed to prepare delete sql:%v\n", err)
		return err

	} else if _, err := stmt.Exec(namespace, jobId, seqno, EXEC_DONE); err != nil {
		Info("failed to prepare delete sql:%v\n", err)
		return err
	}
	return nil
}

func GetJobExecRecords(namespace, jobId string, jobExecView *[]ExecView) error {

	var view ExecView

	rows, err := Database.Query("select seqno, status, endTime from job where namespace =? and jobId = ?", namespace, jobId)
	if err != nil {
		Info("failed to prepare query sql:%v\n", err)
		return err
	}
	/*
		if rows.Next() == false {
			log.Print("No Such JobId! Please Search other Jobs")
			return nil
		}
	*/
	for rows.Next() {
		if err := rows.Scan(&view.SeqNo, &view.EndStatus, &view.EndTime); err != nil {
			log.Println(err)
		}

		fmt.Println(view.SeqNo, view.EndStatus, view.EndTime)
		*jobExecView = append(*jobExecView, view)
	}
	return nil
}

func InsertExecutionRecord(e *Execution) {
	if stmt, err := Database.Prepare("insert into job(namespace, jobId, seqno, progress, status,finished, cancelled, startTime, endTime) values (?,?,?,?,?,?,?,?,?)"); err != nil {
		log.Fatalf("[She-Ra][error] failed to prepare insert sql:%v\n", err)
	} else if _, err := stmt.Exec(e.Namespace, e.JobId, e.SeqNo, e.Progress, e.EndStatus, e.Finished, e.Cancelled, e.StartTime, e.EndTime); err != nil {
		log.Fatalf("[She-Ra][error] failed to insert data into database:%v\n", err)
	}
}

func UpdateExecutionRecord(e *Execution) {
	if stmt, err := Database.Prepare("update job set progress=?, status=?, finished=?, cancelled=?, endTime=? where namespace=? and jobId=? and seqno=?"); err != nil {
		log.Fatalf("[She-Ra][error] failed to prepare update sql:%v\n", err)
	} else if _, err := stmt.Exec(e.Progress, e.EndStatus, e.Finished, e.Cancelled, e.EndTime, e.Namespace, e.JobId, e.SeqNo); err != nil {
		log.Fatalf("[She-Ra][error] failed to update data in database:%v\n", err)
	}
}

func SetExecutionCancelled(namespace, jobId string, seqno int) {
	if stmt, err := Database.Prepare("update job set cancelled=1 where namespace=? and jobId=? and seqno=?"); err != nil {
		log.Fatalf("[She-Ra][error] failed to prepare update sql:%v\n", err)
	} else if _, err := stmt.Exec(namespace, jobId, seqno); err != nil {
		log.Fatalf("[She-Ra][error] failed to set execution cancelled in database:%v\n", err)
	}
}

func SetAllCancelled(namespace, jobId string) {
	if stmt, err := Database.Prepare("update job set cancelled=1 where namespace=? and jobId=?"); err != nil {
		log.Fatalf("[She-Ra][error] failed to prepare update sql:%v\n", err)
	} else if _, err := stmt.Exec(namespace, jobId); err != nil {
		log.Fatalf("[She-Ra][error] failed to set all cancelled in database:%v\n", err)
	}
}

func GetCancelStatus(namespace, jobId string, seqno int32) int32 {
	var cancelStat int32
	if stmt, err := Database.Prepare("select cancelled from job where namespace=? and jobId=? and seqno=?"); err != nil {
		log.Fatalf("[She-Ra][error] failed to prepare update sql:%v\n", err)
	} else if err := stmt.QueryRow(namespace, jobId, seqno).Scan(&cancelStat); err != nil {
		log.Fatalf("[She-Ra][error] failed to get cancel stat in database:%v\n", err)
	}
	return cancelStat
}

func GetFinishStatus(namespace, jobId string, seqno int32) (bool, error) {
	var finished int32
	if stmt, err := Database.Prepare("select finished from job where namespace=? and jobId=? and seqno=?"); err != nil {
		log.Println("error failed to prepare update sql:v\n", err)
		return false, err
	} else if err := stmt.QueryRow(namespace, jobId, seqno).Scan(&finished); err != nil {
		log.Println("error failed to get cancel stat in database:%v\n", err)
		return false, err
	}
	if finished == EXEC_NOT_DONE {
		return false, nil
	} else {
		return true, nil
	}
}

func DeleteJobExecutions(namespace, jobId string) {
	if stmt, err := Database.Prepare("delete from job where namespace=? and jobId=?"); err != nil {
		log.Fatalf("[She-Ra][error] failed to prepare delete sql:%v\n", err)
	} else if _, err := stmt.Exec(namespace, jobId); err != nil {
		log.Fatalf("[She-Ra][error] failed to delete job executions in database:%v\n", err)
	}
}

func getRunningCount(namespace, jobId string, finished int) int {
	var count int
	if stmt, err := Database.Prepare("select count(*) from job where namespace=? and jobId =? and finished=?"); err != nil {
		log.Fatalf("[She-Ra][error] failed to prepare query sql:%v\n", err)
	} else if err := stmt.QueryRow(namespace, jobId, finished).Scan(&count); err != nil {
		log.Fatalf("[She-Ra][error] failed to get the tatoal running executions:%v\n", err)
	}
	return count
}

func Contains(key Key) bool {
	var count int
	if stmt, err := Database.Prepare("select count(*) from job where namespace=? and jobId =?"); err != nil {
		log.Fatalf("[She-Ra][error] failed to prepare query sql:%v\n", err)
	} else if err := stmt.QueryRow(key.Ns, key.Id).Scan(&count); err != nil {
		log.Fatalf("[She-Ra][error] failed to get the tatoal running executions:%v\n", err)
	}

	if count > 0 {
		return true
	} else {
		return false
	}
}

func ReadData(key Key, job *configdata.Job) error {
	fileName := WS_PATH + key.Ns + "/" + key.Id + "/.shera/configfile"
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			Info("%s: File not found.  Creating new file.\n", fileName)

		} else {
			log.Fatalln("[She-Ra][error]Error reading file:", err)

		}

	}

	if err := proto.Unmarshal(data, job); err != nil {
		log.Fatalln("Failed to parse address book:", err)
	}
	return err
}

func WriteData(key Key, job *configdata.Job) error {
	var (
		data []byte
		err  error
		file *os.File
	)

	if data, err = proto.Marshal(job); err != nil {
		log.Fatalf("[She-Ra][error] marshling error:%v\n", err)
		return err
	}

	Info("proto marshal: job", string(data))
	fileName := WS_PATH + key.Ns + "/" + key.Id + "/.shera/configfile"

	if file, err = os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666); err != nil {
		log.Fatalf("[She-Ra][error] failed to open file:%v\n", err)
		return err
	} else {
		Info("OpenFile ", fileName, " successfully.")
	}

	if _, err = file.Write(data); err != nil {
		Info("write data ino file %s failed\n", fileName)
		return err
	} else {
		Info("write data into file succeed. file: ", fileName, "; data: ", string(data))
	}

	if err = file.Close(); err != nil {
		Info("file close failed: ", err)
		return err
	} else {
		Info("file close succeed.")
	}
	return err
}

func FileExists(fileName string) bool {
	var bExist bool
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		log.Print("file is not exist, ", err)
		bExist = false
	} else {
		bExist = true
		log.Print("file is exist.")
	}
	return bExist
}

func getLocalPath() string {
	var filePath string
	_, fullFileName, _, ok := runtime.Caller(0)
	if ok != false {
		filePath = path.Dir(fullFileName)

	}
	log.Print("get path :", filePath)
	return filePath

}

func Info(template string, values ...interface{}) {
	log.Output(2, fmt.Sprintf("[She-Ra][info] "+template+"\n", values...))
}

func ReadFile(fName string, nLen int) (int, string, error) {
	var nSize int64 = int64(nLen)
	file, err := os.Open(fName)
	if err != nil {
		Info("1: %v\n", err)
		return 0, "", err
	}
	defer file.Close()
	buf := make([]byte, 1024)
	len, err := file.ReadAt(buf, nSize)
	fmt.Println("readFile nSize: ", nSize)
	if err != nil && err.Error() != "EOF" {
		Info("2: %v\n", err)
		return 0, "", err
	}
	Info("read data: %s; len: %d.", string(buf), len)
	return int(nSize) + len, string(buf), nil
}

func ReadFileToEnd(fName string, nLen int) (int, string, error) {
	file, err := os.Open(fName)

	var msg string
	if err != nil {
		Info("1: %v\n", err)
		return 0, "", err
	}
	defer file.Close()
	for {
		buf := make([]byte, 1024)
		len, err := file.ReadAt(buf, int64(nLen))
		if err != nil && err != io.EOF {
			Info("2: %v\n", err)
			return 0, "", err
		} else if err == io.EOF {
			msg = msg + string(buf)
			return nLen + len, msg + string(buf), nil
		}
		msg = msg + string(buf)
		Info("read data: %s, len: %d", string(buf), len)
		nLen = nLen + len
	}

}

func WriteFile(path string, name string, content string) (int, error) {
	if len(path) == 0 || len(name) == 0 {
		return 0, errors.New("path or name is null.")
	}
	//fmt.Println(path + name)
	f, err := os.OpenFile(path+name, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0660)
	if err != nil {
		Info("Open file failed: %v\n", err)
		return 0, err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	len, err := w.WriteString(content)
	//fmt.Println("size : ", len)
	w.Flush()
	return len, err
}

func checkFile(fName string) (bool, error) {
	fInfo, err := os.Stat(fName)
	if err != nil {
		return false, err
	}
	if fInfo.Mode().String() == "-rwxrwxrwx" {
		Info("change mode succeed.")
		return true, nil
	} else {
		Info("change mode failed: %v\n", fInfo.Mode().String())
		return false, nil
	}
}

func Client(w http.ResponseWriter, r *http.Request) {

	html := `<html> <head> </head> <body> <script type="text/javascript"> var sock = null; var wsuri = "ws://192.168.56.101:8282/echoLog"; window.onload = function() {
// checkout the browser support
window.WebSocket = window.WebSocket || window.MozWebSocket;
if (!window.WebSocket) {
	alert("WebSocket not supported by this browser.");
	return;
};

var opendiv = document.getElementById("open");
var logdiv = document.getElementById("log");
var datadiv =  document.getElementById("showData");
var clodiv = document.getElementById("close");
logdiv.innerText = "log" + logdiv.innerText;
console.log("onload");
sock = new WebSocket(wsuri);
sock.onopen = function(){
	sendMsg("open");
	opendiv.innerText = "websocket open: build connect." + opendiv.innerText;
};
sock.onmessage = function(e){
	datadiv.innerText = datadiv.innerText + e.data;
};
sock.onclose = function(e){
	clodiv.innerText = "close connect.";
};
window.onbeforeunload = function() {
	closeSocket();
};

};
function send(bFlag, seq) {
	//var msg = document.getElementById('message').value;
	sock.send(JSON.stringify({
		namespace: "wangshuaijian",
		jobid:	   "test001",
		seqno:	   seq,
		flag:	   bFlag
	}));
};

function sendMsg(msg){
	seq = document.getElementById('message').value;
	sock.send(msg, seq);
};
function closeSocket(){
	sock.close();
};
function start(){
	var bool = true;
	//alert("start");
	seq = document.getElementById('message').value;
	//alert(seq)
	send(bool, seq);
};
function sendClose() {
	var bool = false;
	seq = document.getElementById('message').value;
	send(bool, seq);
	sock.close();
};
</script>
 <h1>WebSocket Echo Test</h1> <form> <p>Message: <input id="message" type="text" value="seq no"></p> </form>
<button onclick="sendClose();">Send close Message</button>
<button onclick="closeSocket();"> close Socket </button>
<button onclick="start();"> start Socket </button>
<textarea>jion</textarea>
<div id="open"></div><div id="log"></div><div id="showData"></div>
<div id="close"></div>
 </body> </html>`
	io.WriteString(w, html)
}

func WatchFile(start chan bool, end chan bool, fName string, ws *websocket.Conn) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
		ws.Close()
		return
	}
	defer watcher.Close()
	done := make(chan bool)

	err = watcher.Add(fName)
	if err != nil {
		log.Fatal("when add file to watcher:", err)
		ws.Close()
		return
	}

	go func(start chan bool, end chan bool, done chan bool, file string, ws *websocket.Conn) {
		var nLen int = 0
		<-start
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write {
					len, msg, err := ReadFile(file, nLen)

					nLen = len
					if err = websocket.Message.Send(ws, msg); err != nil {
						Info("send msg: %v\n", err)
					}
				} else if event.Op&fsnotify.Chmod == fsnotify.Chmod {
					_, msg, err := ReadFileToEnd(file, nLen)
					if err = websocket.Message.Send(ws, msg); err != nil {
						Info("chmod and send msg to client, %v\n", err)
					}
					done <- true
					return
				}
			case err := <-watcher.Errors:
				log.Println("---------error:", err)
			case <-end:
				Info("----receive end channel \n")
				done <- true
				return

			}
		}
	}(start, end, done, fName, ws)

	<-done
	close(done)
	Info("end ws close")
	ws.Close()
}
