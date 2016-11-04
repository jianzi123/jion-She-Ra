package utils

import (
	"database/sql"
	"github.com/golang/protobuf/proto"
	"github.com/haveatry/She-Ra/configdata"
	"github.com/magiconair/properties"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"log"
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
	dbPath := props.MustGet("database.path")
	if Database, err = sql.Open("sqlite3", dbPath); err != nil {
		info("failed to setup database")
	}

	//create table job to store execution information
	sql := `Create table IF NOT EXISTS job(namespace CHAR(100) NOT NULL, jobId CHAR(100) NOT NULL,seqno integer NOT NULL,progress integer,status integer, finished integer, cancelled integer, startTime integer, endTime integer, PRIMARY KEY(namespace, jobId, seqno))`
	if _, err = Database.Exec(sql); err != nil {
		info("failed to create table job")
	}
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

func DeleteExecutionRecord(namespace, jobId string, seqno int32) {
	if stmt, err := Database.Prepare("delete from job where namespace=? and jobId=? and seqno=?"); err != nil {
		log.Fatalf("[She-Ra][error] failed to prepare delete sql:%v\n", err)
	} else if _, err := stmt.Exec(namespace, jobId, seqno); err != nil {
		log.Fatalf("[She-Ra][error] failed to delete job execution in database:%v\n", err)
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
			info("%s: File not found.  Creating new file.\n", fileName)

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

	info("proto marshal: job", string(data))
	fileName := WS_PATH + key.Ns + "/" + key.Id + "/.shera/configfile"

	if file, err = os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666); err != nil {
		log.Fatalf("[She-Ra][error] failed to open file:%v\n", err)
		return err
	} else {
		info("OpenFile ", fileName, " successfully.")
	}

	if _, err = file.Write(data); err != nil {
		info("write data ino file %s failed\n", fileName)
		return err
	} else {
		info("write data into file succeed. file: ", fileName, "; data: ", string(data))
	}

	if err = file.Close(); err != nil {
		info("file close failed: ", err)
		return err
	} else {
		info("file close succeed.")
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

func info(template string, values ...interface{}) {
	log.Printf("[She-Ra][info] "+template+"\n", values...)
}
