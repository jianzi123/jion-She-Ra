package jobs

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"github.com/emicklei/go-restful"
	"github.com/haveatry/She-Ra/configdata"
	"github.com/haveatry/She-Ra/lru"
	. "github.com/haveatry/She-Ra/utils"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/websocket"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type JobManager struct {
	JobCache     *lru.ARCCache
	SeqNo        map[Key]int
	ExecChan     map[Key]chan int
	KillExecChan map[Key]chan int
	WaitExec     map[Key]*sync.WaitGroup
	db           *sql.DB
	//Content      string
	accessLock *sync.RWMutex
}

type JobCommand struct {
	Name string
	Args []string
}

func (cmd *JobCommand) Exec() (bool, string) {
	var (
		cmdOut []byte
		err    error
	)

	if cmdOut, err = exec.Command(cmd.Name, cmd.Args...).Output(); err != nil {
		Info("Failed to execute command: (cmd=%s, agrs=%v): \\n%v\\n", cmd.Name, cmd.Args)
		return false, ""
	}

	Info("Output (cmd=%s, agrs=%v): \\n%v\\n", cmd.Name, cmd.Args, string(cmdOut))
	return true, string(cmdOut)
}

func (cmd *JobCommand) ExecPipeCmd(in *JobCommand) int {
	var err error

	producer := exec.Command(in.Name, in.Args...)
	consumer := exec.Command(cmd.Name, cmd.Args...)
	if consumer.Stdin, err = producer.StdoutPipe(); err != nil {
		Info("Failed to combine the 2 commands with pipe\n")
		return EXEC_ERROR
	}

	if err = consumer.Start(); err != nil {
		Info("err occurred when start executing command: (cmd=%s, agrs=%v): \\n%v\\n", cmd.Name, cmd.Args)
		return EXEC_ERROR
	}

	if err = producer.Run(); err != nil {
		Info("err occurred when executing command: (cmd=%s, agrs=%v): \\n%v\\n", in.Name, in.Args)
		return EXEC_ERROR
	}

	if err = consumer.Wait(); err != nil {
		Info("err occurred when waiting the command executing complete: (cmd=%s, agrs=%v): \\n%v\\n", cmd.Name, cmd.Args)
		return EXEC_ERROR
	}

	return EXEC_FINISHED

}

func NewJobManager() (*JobManager, error) {
	if cache, err := lru.NewARC(100); err != nil {
		return nil, errors.New("init lru cache failed")
	} else {
		jobManager := &JobManager{
			JobCache:     cache,
			SeqNo:        make(map[Key]int, 10240),
			ExecChan:     make(map[Key]chan int, 10240),
			KillExecChan: make(map[Key]chan int, 10240),
			WaitExec:     make(map[Key]*sync.WaitGroup, 10240),
			db:           Database,
			accessLock:   &sync.RWMutex{},
		}
		if err = jobManager.recoverSeqNo(); err != nil {
			return nil, errors.New("failed to recover seqno from database")
		}
		return jobManager, nil
	}
}

func jobExists(key Key, cache *lru.ARCCache) bool {
	//check the job cache at first
	if cache.Contains(key) {
		return true
	}

	//take further check from the job config file
	if FileExists(WS_PATH + key.Ns + "/" + key.Id + "/.shera/configfile") {
		return true
	}

	return false
}

func (d *JobManager) createJob(request *restful.Request, response *restful.Response) {
	Info("Enter createJob\n")
	ns := request.PathParameter("namespace")
	job := configdata.Job{}
	//waitGroup := new(sync.WaitGroup)
	if err := request.ReadEntity(&job); err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}

	job.MaxKeepDays = MAX_KEEP_DAYS
	job.MaxExecutionRecords = MAX_EXEC_NUM

	key := Key{Ns: ns, Id: job.Id}
	if jobExists(key, d.JobCache) {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, "job already exists")
		return
	}

	d.JobCache.Add(key, job)

	d.accessLock.Lock()
	d.SeqNo[key] = 0
	d.accessLock.Unlock()

	createWorkSpace := &JobCommand{
		Name: "mkdir",
		Args: []string{"-p", WS_PATH + key.Ns + "/" + key.Id + "/.shera/" + EXECUTION_PATH},
	}
	createWorkSpace.Exec()
	//create Dockerfile
	DockerfilePath := WS_PATH + key.Ns + "/" + key.Id + "/.shera/"
	DockerfileName := "Dockerfile"
	DockerfileContent := job.ImgManager.DockerFileContent

	WriteFile(DockerfilePath, DockerfileName, DockerfileContent)

	//encode job info and store job info into config file
	if err := WriteData(key, &job); err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}

	if err := InsertJobViewRecord(key.Ns, key.Id); err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return

	}

	response.WriteHeaderAndEntity(http.StatusCreated, &job)
}

func (d *JobManager) readJob(request *restful.Request, response *restful.Response) {
	var job configdata.Job
	jobId := request.PathParameter("job-id")
	ns := request.PathParameter("namespace")
	key := Key{Ns: ns, Id: jobId}
	Info("namespace is %s, job.Id: %s\n", ns, jobId)

	if !jobExists(key, d.JobCache) {
		Info("failed to find the job %s\n", jobId)
		response.WriteHeader(http.StatusNotFound)
	} else if value, OK := d.JobCache.Get(key); OK {
		job = value.(configdata.Job)
		Info("Get job successfully")
		response.WriteHeaderAndEntity(http.StatusFound, &job)
	} else if FileExists(WS_PATH + key.Ns + "/" + key.Id + "/.shera/configfile") {
		if err := ReadData(key, &job); err != nil {
			Info("failed to read job config data")
			response.WriteHeader(http.StatusNotFound)
		} else {
			d.JobCache.Add(key, job)
			response.WriteHeaderAndEntity(http.StatusFound, &job)
		}
	} else {
		Info("failed to find the job %s\n", jobId)
		response.WriteHeader(http.StatusNotFound)
	}
	return
}

func (d *JobManager) readAllJobs(request *restful.Request, response *restful.Response) {
	namespace := request.PathParameter("namespace")
	//jobId := request.PathParameter("job-id")
	Info("namespace: %s ", namespace)

	var jobView []JobView
	Info("before jobView.len()=%d", len(jobView))
	err := GetJobViewRecords(namespace, &jobView)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return

	}

	Info("after jobView.len()=%d", len(jobView))
	response.WriteHeaderAndEntity(http.StatusCreated, jobView)

}

func (d *JobManager) updateJob(request *restful.Request, response *restful.Response) {
	newJob := configdata.Job{}
	err := request.ReadEntity(&newJob)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}

	ns := request.PathParameter("namespace")
	key := Key{Ns: ns, Id: newJob.Id}
	if jobExists(key, d.JobCache) {
		d.JobCache.Add(key, newJob)

		//encode job info and store job info into config file
		if err = WriteData(key, &newJob); err != nil {
			response.AddHeader("Content-Type", "text/plain")
			response.WriteErrorString(http.StatusInternalServerError, err.Error())
			return
		}

		response.WriteHeaderAndEntity(http.StatusAccepted, &newJob)
	} else {
		//waitGroup := new(sync.WaitGroup)
		d.JobCache.Add(key, newJob)

		d.accessLock.Lock()
		d.SeqNo[key] = 0
		//d.ExecChan[key] = make(chan int, 1)
		//d.KillExecChan[key] = make(chan int, 1)
		//d.WaitExec[key] = waitGroup
		d.accessLock.Unlock()

		createWorkSpace := &JobCommand{
			Name: "mkdir",
			Args: []string{"-p", WS_PATH + key.Ns + "/" + key.Id + "/.shera/" + EXECUTION_PATH},
		}
		createWorkSpace.Exec()

		//encode job info and store job info into config file
		if err = WriteData(key, &newJob); err != nil {
			response.AddHeader("Content-Type", "text/plain")
			response.WriteErrorString(http.StatusInternalServerError, err.Error())
			return
		}

		response.WriteHeaderAndEntity(http.StatusCreated, &newJob)

	}
	return
}

func (d *JobManager) delJob(request *restful.Request, response *restful.Response) {
	ns := request.PathParameter("namespace")
	jobId := request.PathParameter("job-id")
	key := Key{Ns: ns, Id: jobId}

	SetAllCancelled(key.Ns, key.Id)

	//Need to kill runnig execution of this job
	go func() {
		if _, OK := d.KillExecChan[key]; !OK {
			d.KillExecChan[key] = make(chan int, 1)
		}
		d.KillExecChan[key] <- EXEC_KILL_ALL
	}()

	//wait until all the running executions exit
	d.WaitExec[key].Wait()

	DeleteJobExecutions(key.Ns, key.Id)

	cleanupCmd := &JobCommand{
		Name: "rm",
		Args: []string{"-rf", WS_PATH + ns + "/" + jobId},
	}

	success, _ := cleanupCmd.Exec()
	if !success {
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	go func() {
		//read KillExecChan again to ensure it unblocked
		<-d.KillExecChan[key]
	}()

	d.JobCache.Remove(key)

	d.accessLock.Lock()
	Info("delJob:get access lock successfully")
	close(d.KillExecChan[key])
	close(d.ExecChan[key])
	delete(d.SeqNo, key)
	delete(d.ExecChan, key)
	delete(d.KillExecChan, key)
	delete(d.WaitExec, key)
	d.accessLock.Unlock()

	response.WriteHeader(http.StatusAccepted)
}

func (d *JobManager) execJob(request *restful.Request, response *restful.Response) {
	ns := request.PathParameter("namespace")
	jobId := request.PathParameter("job-id")
	key := Key{Ns: ns, Id: jobId}
	job := configdata.Job{}

	Info("execJob: key.ns=%s, key.id=%s\n", key.Ns, key.Id)

	if !d.JobCache.Contains(key) &&
		FileExists(WS_PATH+key.Ns+"/"+key.Id+"/.shera/configfile") {
		if err := ReadData(key, &job); err != nil {
			Info("failed to read job config data")
			response.WriteHeader(http.StatusNotFound)
		} else {
			d.JobCache.Add(key, job)
		}

	}

	if jobExists(key, d.JobCache) {
		d.accessLock.Lock()
		jobExec := &Execution{
			Namespace: key.Ns,
			JobId:     key.Id,
			SeqNo:     int32(d.SeqNo[key] + 1),
			Progress:  EXEC_INIT,
			EndStatus: EXEC_FAILURE,
			Finished:  EXEC_NOT_DONE,
			Cancelled: EXEC_NOT_CANCELLED,
			StartTime: time.Now().Unix(),
			EndTime:   0,
		}
		//now := time.Now()
		//year, mon, day := now.Date()
		//hour, min, sec := now.Clock()
		//jobExec.LogFile = fmt.Sprintf("%03d-%d%02d%02d%02d%02d%02d", int(jobExec.Number), year, mon, day, hour, min, sec)
		response.WriteHeaderAndEntity(http.StatusCreated, jobExec)

		d.SeqNo[key] = d.SeqNo[key] + 1
		Info("insertRecord: key.ns=%s, key.id=%s, jobExec.SeqNo=%d\n", key.Ns, key.Id, jobExec.SeqNo)
		InsertExecutionRecord(jobExec)
		UpdateJobViewStartTime(key.Ns, key.Id, jobExec.StartTime)
		Info("Get the write lock successfully")
		var OK bool
		if _, OK = d.WaitExec[key]; !OK {
			d.WaitExec[key] = new(sync.WaitGroup)
		}

		if _, OK = d.ExecChan[key]; !OK {
			d.ExecChan[key] = make(chan int, 1)
		}

		d.WaitExec[key].Add(1)
		go d.runJobExecution(key, jobExec.SeqNo)
		d.accessLock.Unlock()
		return
	} else {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, "no such job found")
		return
	}
}

func (d *JobManager) runJobExecution(key Key, seqno int32) {
	var retCode int
	d.ExecChan[key] <- EXEC_GOROUTINE
	Info("runJobExecution key.Ns=%s, key.Id=%s, seqno=%d\n", key.Ns, key.Id, seqno)
	if value, OK := d.JobCache.Get(key); OK {
		if cancelStat := GetCancelStatus(key.Ns, key.Id, seqno); cancelStat == EXEC_CANCELLED {
			Info("GetCancelStatus key.Ns=%s, key.Id=%s, seqno=%d cancelled\n", key.Ns, key.Id, seqno)
			goto COMMON_HANDLING
		}

		job := value.(configdata.Job)
		progress := EXEC_INIT
		Info("key.Ns=%s, key.Id=%s, seqno=%d begin to execute command", key.Ns, key.Id, seqno)

		//change the working dir
		targetPath, err := filepath.Abs(WS_PATH + key.Ns + "/" + key.Id)
		if err != nil {
			Info("AbsError (%s): %s\\n", WS_PATH+key.Ns+"/"+key.Id, err)
			goto COMMON_HANDLING

		}

		Info("Target Path: %s\\n", targetPath)
		err = os.Chdir(targetPath)
		if err != nil {
			Info("ChdirError (%s): %s\\n", targetPath, err)
			goto COMMON_HANDLING
		}

		jdkVersion := job.JdkVersion

		fPath := fmt.Sprintf("%s%s/%s/.shera/%s", WS_PATH, key.Ns, key.Id, EXECUTION_PATH)
		lId := strconv.Itoa(int(seqno))
		//pull code from git
		if codeManager := job.GetCodeManager(); codeManager != nil && codeManager.GitConfig != nil {
			Info("key.Ns=%s, key.Id=%s, seqno=%d begin to pulling code\n", key.Ns, key.Id, seqno)
			progress = EXEC_CODE_PULLING
			err = os.RemoveAll(WS_PATH + key.Ns + "/" + key.Id + "/.git/")
			if err != nil {
				Info("err occurred when remove .git dir: %v", err)
			}
			gitInitCmd := &JobCommand{
				Name: "git",
				Args: []string{"init"},
			}
			if retCode = gitInitCmd.ExecAsync(d, key, seqno, progress, jdkVersion); retCode != EXEC_FINISHED {
				goto COMMON_HANDLING
			}

			gitConfigCmd := &JobCommand{
				Name: "git",
				Args: []string{"config", "remote.origin.url", codeManager.GitConfig.Repo.Url},
			}
			if retCode = gitConfigCmd.ExecAsync(d, key, seqno, progress, jdkVersion); retCode != EXEC_FINISHED {
				goto COMMON_HANDLING
			}

			gitPullCmd := &JobCommand{
				Name: "git",
				Args: []string{"pull", "origin", codeManager.GitConfig.Branch},
			}
			if retCode = gitPullCmd.ExecAsync(d, key, seqno, progress, jdkVersion); retCode != EXEC_FINISHED {
				goto COMMON_HANDLING
			}
			// add job step finish to db and write to file.
			fline := fmt.Sprintf("\n step %d finished. \n", progress)
			WriteFile(fPath, lId, fline)
		}

		if buildManager := job.GetBuildManager(); buildManager != nil {
			progress = EXEC_CODE_BUILDING
			var i, j, k int
			for _, v := range buildManager.SeqNo {
				switch v {
				case configdata.BuildManager_NONE:
					Info("Bad BuildManager data\n")
					goto COMMON_HANDLING

				case configdata.BuildManager_ANT:
					if buildManager.AntConfig[i] == nil {
						Info("Bad BuildManager data\n")
						goto COMMON_HANDLING

					} else {
						antBuildCmd := &JobCommand{
							Name: "ant",
							Args: []string{"-f", buildManager.AntConfig[i].BuildFile, "-D" + buildManager.AntConfig[i].Properties},
						}
						retCode = antBuildCmd.ExecAsync(d, key, seqno, progress, jdkVersion)
						if retCode != EXEC_FINISHED {
							goto COMMON_HANDLING

						}
						i++
					}
				case configdata.BuildManager_MVN:
					if buildManager.MvnConfig[j] == nil {
						Info("Bad BuildManager data\n")
						goto COMMON_HANDLING

					} else {
						mvnBuildCmd := &JobCommand{
							Name: "mvn",
							Args: []string{"-f", buildManager.MvnConfig[j].Pom, buildManager.MvnConfig[j].Goals},
						}

						retCode = mvnBuildCmd.ExecAsync(d, key, seqno, progress, jdkVersion)
						if retCode != EXEC_FINISHED {

							goto COMMON_HANDLING
						}
						j++

					}
				case configdata.BuildManager_SHELL:
					if buildManager.Cmd[k] == "" {
						Info("Bad BuildManager data\n")
						goto COMMON_HANDLING

					} else {
						Info("In build manager: %s\n", buildManager.Cmd[k])
						cmdWithArgs := strings.Split(buildManager.Cmd[k], " ")
						shellCmd := &JobCommand{
							Name: cmdWithArgs[0],
							Args: cmdWithArgs[1:],
						}
						retCode = shellCmd.ExecAsync(d, key, seqno, progress, jdkVersion)
						if retCode != EXEC_FINISHED {
							goto COMMON_HANDLING

						}
						k++

					}
				default:
					goto COMMON_HANDLING
				}
			}

			// add job step finish to db and write to file.
			fline := fmt.Sprintf("\n step %d finished. \n", progress)
			WriteFile(fPath, lId, fline)
		}

		if imgManager := job.GetImgManager(); imgManager != nil {
			progress = EXEC_IMAGE_BUILDING
			imgBuildTime := strings.Replace(strings.ToLower((strings.Split(time.Now().Format(time.RFC3339), "+"))[0]), ":", "-", 2)
			tag := IMG_REGISTRY + "/she-ra/" + imgManager.ImgName + ":" + imgBuildTime
			Info("%s\n", tag)
			imgBuildCmd := &JobCommand{
				Name: "docker",
				Args: []string{
					"build",
					"-t",
					tag,
					"-f",
					imgManager.DockerFile,
					".",
				},
			}

			retCode = imgBuildCmd.ExecAsync(d, key, seqno, progress, jdkVersion)
			if retCode != EXEC_FINISHED {
				goto COMMON_HANDLING
			}

			var imgId string
			getImgNoCmd := JobCommand{
				Name: "tail",
				Args: []string{"-n", "1", fPath + lId},
			}
			if success, result := getImgNoCmd.Exec(); success {

				resultV := strings.Split(result, " ")
				if len(resultV) != 3 || resultV[0] != "Successfully" {
					Info("failed to get correct img Id: %s\n", result)
					goto COMMON_HANDLING
				}
				imgId = strings.Replace(resultV[2], "\n", "", 1)
			} else {
				goto COMMON_HANDLING
			}
			Info("Image Id=%s", imgId)

			progress = EXEC_IMAGE_PUSHING
			imgPushCmd := JobCommand{
				Name: "docker",
				Args: []string{"push", tag},
			}
			retCode = imgPushCmd.ExecAsync(d, key, seqno, progress, jdkVersion)
			if retCode != EXEC_FINISHED {
				goto COMMON_HANDLING
			}

			imgRemoveCmd := JobCommand{
				Name: "docker",
				Args: []string{"rmi", tag},
			}
			retCode = imgRemoveCmd.ExecAsync(d, key, seqno, progress, jdkVersion)
			if retCode != EXEC_FINISHED {
				goto COMMON_HANDLING
			}

			//sql := "insert into image (create_time,creator,image_id,image_type,name,remark,resource_name,version,is_base_image,is_delete) values (" + "\"" + strings.Replace((strings.Split(time.Now().Format(time.RFC3339), "+"))[0], "T", " ", 1) + "\",1,\"" + imgId + "\",1,\"She-Ra/" + imgManager.ImgName + "\",\"She-Ra\",\"" + imgManager.ImgName + ".war" + "\",\"" + imgBuildTime + "\",2,0);"
			sql := fmt.Sprintf("insert into image (create_time,creator,image_id,image_type,name,remark,resource_name,version,is_base_image,is_delete) values ('%s', 1, '%s', 1, '%s','She-Ra','%s','%s',2,0)",
				strings.Replace((strings.Split(time.Now().Format(time.RFC3339), "+"))[0], "T", " ", 1),
				imgId, "She-Ra/"+imgManager.ImgName, imgManager.ImgName+".war", imgBuildTime)
			Info("%s\n", sql)
			writeDBCmd := JobCommand{
				Name: "bash",
				Args: []string{
					"-c",
					"echo " + "\"" + sql + "\"" + " | mysql -h" + IMG_HOST + " -u" + IMG_USER + " -p" + IMG_PASSWD + " " + IMG_DBNAME,
				},
			}

			Info("writeDBCmd is %v\n", writeDBCmd)
			retCode = writeDBCmd.ExecAsync(d, key, seqno, progress, jdkVersion)
			if retCode != EXEC_FINISHED {
				goto COMMON_HANDLING
			}

		}

		jobExec := &Execution{
			Namespace: key.Ns,
			JobId:     key.Id,
			SeqNo:     seqno,
			Progress:  progress,
			EndStatus: EXEC_SUCCESS,
			Finished:  EXEC_DONE,
			Cancelled: EXEC_NOT_CANCELLED,
			StartTime: 0,
			EndTime:   time.Now().Unix(),
		}
		UpdateExecutionRecord(jobExec)
		UpdateJobViewStatus(jobExec.Namespace, jobExec.JobId, jobExec.EndTime, jobExec.EndStatus)
		if err := os.Chmod(fPath+lId, 0777); err != nil {
			Info("job finished normally, when changing mode %v\n", err)
		}

	}

COMMON_HANDLING:
	d.accessLock.Lock()
	<-d.ExecChan[key]
	d.WaitExec[key].Done()
	d.accessLock.Unlock()
	return
}

//watch one job execution status change
func (d *JobManager) watchJobExecution(request *restful.Request, response *restful.Response) {

}

//open on job execution record
func (d *JobManager) openJobExecution(request *restful.Request, response *restful.Response) {

}

//get the job execution list
func (d *JobManager) getAllJobExecutions(request *restful.Request, response *restful.Response) {
	namespace := request.PathParameter("namespace")
	jobId := request.PathParameter("job-id")
	Info("namespace: %s jobId:%s", namespace, jobId)

	var jobExecView []ExecView
	//jobExecView := make([]ExecView, 0, 20)
	Info("before jobExecView.len()=%d", len(jobExecView))
	err := GetJobExecRecords(namespace, jobId, &jobExecView)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return

	}

	Info("after jobExecView.len()=%d", len(jobExecView))
	response.WriteHeaderAndEntity(http.StatusCreated, jobExecView)
}

//delete one job execution record
func (d *JobManager) delJobExecution(request *restful.Request, response *restful.Response) {
	namespace := request.PathParameter("namespace")
	jobId := request.PathParameter("job-id")
	executionId := request.PathParameter("execution_id")

	if seqno, err := strconv.Atoi(executionId); err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, "please provide one valid seqno")

	} else {

		if finished, err := GetFinishStatus(namespace, jobId, int32(seqno)); err != nil {
			response.AddHeader("Content-Type", "text/plain")
			response.WriteErrorString(http.StatusInternalServerError, "failed to the status of this execution")

		} else if finished {

			if err := DelJobExecRecord(namespace, jobId, seqno); err != nil {
				response.AddHeader("Content-Type", "text/plain")
				response.WriteErrorString(http.StatusInternalServerError, "failed to delete job execution")

			} else {
				response.WriteHeader(http.StatusAccepted)

			}

		} else {
			response.AddHeader("Content-Type", "text/plain")
			response.WriteErrorString(http.StatusInternalServerError, "it is not allowed to delete one running execution")

		}
	}
	return

}

//force stop one job execution
func (d *JobManager) killJobExecution(request *restful.Request, response *restful.Response) {
	Info("Enter killJobExecution\n")
	ns := request.PathParameter("namespace")
	jobId := request.PathParameter("job-id")
	if seqno, err := strconv.Atoi(request.PathParameter("execution_id")); err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	} else {
		key := Key{Ns: ns, Id: jobId}
		Info("killJobExecution key.Ns=%s key.Id=%s\n", key.Ns, key.Id)
		//update the execution as cancelled in the database
		SetExecutionCancelled(key.Ns, key.Id, seqno)
		go func() {
			if _, OK := d.KillExecChan[key]; !OK {
				Info("Init d.KillExecChan\n")
				d.KillExecChan[key] = make(chan int, 1)
			}
			Info("write to d.KillExecChan\n")
			d.KillExecChan[key] <- seqno
		}()

		response.WriteHeader(http.StatusAccepted)
		return
	}
}

func (cmd *JobCommand) ExecAsync(d *JobManager, key Key, seqno, progress int32, jdkVersion string) int {

	var recvCode int
	jobCmd := exec.Command(cmd.Name, cmd.Args...)
	timeout := time.Duration(300 * time.Second)

	stdout, err := jobCmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
		return EXEC_ERROR
	}

	errout, err := jobCmd.StderrPipe()
	if err != nil {
		Info("get stderr failed:%v\n", err)
		return EXEC_ERROR
	}

	jobCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	//select jdkVersion
	if progress == EXEC_CODE_BUILDING && jdkVersion != "" {

		var jdkPath string
		err := Database.QueryRow("select installpath from jdk where version = ?", jdkVersion).Scan(&jdkPath)
		if err != nil {
			if err == sql.ErrNoRows {
				Info("No Result!")

			} else {
				Info("err occurred when select jdkPath :%s", err)
			}
			return EXEC_ERROR

		}
		jobCmd.Env = append(jobCmd.Env, "PATH="+jdkPath+":"+os.Getenv("PATH"))
		Info("******************************************************")
		Info("%s", jobCmd.Env)
		//      Info("%s", os.Environ)
		Info("*****************************************************")

	}

	err = jobCmd.Start()
	if err != nil {
		Info("err occurred when start executing command: (cmd=%s, agrs=%v): \\n%v\\n", cmd.Name, cmd.Args)
		jobExec := &Execution{
			Namespace: key.Ns,
			JobId:     key.Id,
			SeqNo:     seqno,
			Progress:  progress,
			EndStatus: EXEC_FAILURE,
			Finished:  EXEC_DONE,
			Cancelled: EXEC_NOT_CANCELLED,
			StartTime: 0,
			EndTime:   time.Now().Unix(),
		}
		fPath := fmt.Sprintf("%s%s/%s/.shera/%s", WS_PATH, key.Ns, key.Id, EXECUTION_PATH)
		lId := strconv.Itoa(int(seqno))
		fline := fmt.Sprintf("\nFailed in %d. \n", progress)
		WriteFile(fPath, lId, fline)
		if err := os.Chmod(fPath+lId, 0777); err != nil {
			Info("when changing mode, %v\n", err)
		}

		UpdateExecutionRecord(jobExec)
		UpdateJobViewStatus(jobExec.Namespace, jobExec.JobId, jobExec.EndTime, jobExec.EndStatus)
		return EXEC_ERROR
	}

	done := make(chan error)
	go func(key Key, number int, progress int, stdout io.ReadCloser, errout io.ReadCloser) {

		fPath := fmt.Sprintf("%s%s/%s/.shera/%s", WS_PATH, key.Ns, key.Id, EXECUTION_PATH)
		lId := strconv.Itoa(number)
		//Info("waiting for command to finish.")
		reader := bufio.NewReader(stdout)
		erreader := bufio.NewReader(errout)
		for {
			line, err := reader.ReadString('\n')
			if err != nil || io.EOF == err {
				//Info("exec cmd finished. %v\n", err)
				break
			}
			WriteFile(fPath, lId, line)
		}
		for {
			line, err := erreader.ReadString('\n')
			if err != nil || io.EOF == err {
				//Info("get error out finished: %v\n", err)
				break
			}
			WriteFile(fPath, lId, line)
		}
		done <- jobCmd.Wait()
	}(key, int(seqno), int(progress), stdout, errout)

	for {
		select {
		case recvCode = <-d.KillExecChan[key]:
			Info("received kill execution command %d, seqno:%d\n", recvCode, seqno)
			if recvCode == int(seqno) || recvCode == EXEC_KILL_ALL {
				Info("begin to kill the execution command\n")
				pgid, err := syscall.Getpgid(jobCmd.Process.Pid)
				if err == nil {
					syscall.Kill(-pgid, syscall.SIGTERM)
				}
				Info("kill the execution command successfully\n")
				jobExec := &Execution{
					Namespace: key.Ns,
					JobId:     key.Id,
					SeqNo:     seqno,
					Progress:  progress,
					EndStatus: EXEC_FAILURE,
					Finished:  EXEC_DONE,
					Cancelled: EXEC_CANCELLED,
					StartTime: 0,
					EndTime:   time.Now().Unix(),
				}
				UpdateExecutionRecord(jobExec)
				UpdateJobViewStatus(jobExec.Namespace, jobExec.JobId, jobExec.EndTime, jobExec.EndStatus)
				return recvCode
			}
			break

		case <-time.After(timeout):
			Info(" timeout when executing the command:%v\n", cmd)
			pgid, err := syscall.Getpgid(jobCmd.Process.Pid)
			if err == nil {
				syscall.Kill(-pgid, syscall.SIGKILL)
			}
			Info("kill the execution command successfully\n")
			jobExec := &Execution{
				Namespace: key.Ns,
				JobId:     key.Id,
				SeqNo:     seqno,
				Progress:  progress,
				EndStatus: EXEC_FAILURE,
				Finished:  EXEC_DONE,
				Cancelled: EXEC_TIMEOUT,
				StartTime: 0,
				EndTime:   time.Now().Unix(),
			}
			UpdateExecutionRecord(jobExec)
			UpdateJobViewStatus(jobExec.Namespace, jobExec.JobId, jobExec.EndTime, jobExec.EndStatus)
			return int(EXEC_TIMEOUT)

		case err = <-done:
			if err != nil {
				Info("process done with error = %v\n", err)
				jobExec := &Execution{
					Namespace: key.Ns,
					JobId:     key.Id,
					SeqNo:     seqno,
					Progress:  progress,
					EndStatus: EXEC_FAILURE,
					Finished:  EXEC_DONE,
					Cancelled: EXEC_NOT_CANCELLED,
					StartTime: 0,
					EndTime:   time.Now().Unix(),
				}
				UpdateExecutionRecord(jobExec)
				UpdateJobViewStatus(jobExec.Namespace, jobExec.JobId, jobExec.EndTime, jobExec.EndStatus)
				fPath := fmt.Sprintf("%s%s/%s/.shera/%s", WS_PATH, key.Ns, key.Id, EXECUTION_PATH)
				lId := strconv.Itoa(int(seqno))
				fline := fmt.Sprintf("\nFailed in %d. \n", progress)
				WriteFile(fPath, lId, fline)
				if err := os.Chmod(fPath+lId, 0777); err != nil {
					Info("when changing mode, %v\n", err)
				}

				return EXEC_ERROR
			} else {
				return EXEC_FINISHED
			}

		}
	}
}

func (d *JobManager) recoverSeqNo() error {
	rows, err := Database.Query("SELECT namespace, jobId, max(seqno) FROM job group by namespace, jobId")
	if err != nil {
		return err
	} else {
		for rows.Next() {
			var key Key
			var seqno int32
			err = rows.Scan(&key.Ns, &key.Id, &seqno)
			if err != nil {
				return err
			}
			d.accessLock.Lock()
			d.SeqNo[key] = int(seqno)
			d.accessLock.Unlock()
		}
		return nil
	}
}

func (d *JobManager) Log(ws *websocket.Conn) {
	start := make(chan bool)
	end := make(chan bool)
	var boolean bool = false
	for {
		var data JobSock
		//if err := websocket.Message.Receive(ws, &data); err != nil {
		if err := websocket.JSON.Receive(ws, &data); err != nil {
			if err.Error() == "EOF" {
				if boolean == false {
					Info("log job not start and end.")
					ws.Close()
					break
				}
				end <- false
				break
			}
		}
		if data.Flag == true {
			if boolean == true {
				Info("log already start")
				continue
			} else {
				boolean = true
			}
			fName := fmt.Sprintf("%s%s/%s/.shera/%s%s", WS_PATH, data.NameSpace, data.JobId, EXECUTION_PATH, data.SeqNo)
			seqno, err := strconv.Atoi(data.SeqNo)
			if err != nil {
				Info("----convent seqno to string failed: %v\n", err)
				ws.Close()
				return
			}
			key := Key{Ns: data.NameSpace, Id: data.JobId}
			if v, ok := d.SeqNo[key]; ok != true {
				Info("job key not in jobMng")
				ws.Close()
				return
			} else if v < seqno {
				Info("job seqno is not used.")
				ws.Close()
				return
			}
			if flag, err := GetFinishStatus(data.NameSpace, data.JobId, int32(seqno)); err != nil {
				Info("----get finish status failed: %v\n", err)
				ws.Close()
				return
			} else if flag == true {
				buf, err := ioutil.ReadFile(fName)
				if err = websocket.Message.Send(ws, string(buf)); err != nil {
					Info("send Message: %v\n", err)
				}
				Info("-----readFile all.\n")
				ws.Close()
				return
			}
			buf, err := ioutil.ReadFile(fName)
			if err != nil {
				Info("before read data dynamicly, %v\n", err)
				return
			}
			if err = websocket.Message.Send(ws, string(buf)); err != nil {
				Info("before read data dynamicly, read data from file %v\n ", err)
			}

			go WatchFile(start, end, fName, ws)
			start <- true
		}

	}
}

func createJdk(request *restful.Request, response *restful.Response) {
	//      jdkVersion := request.PathParameter("JdkVersion")
	//      jdkPath := request.PathParameter("JdkInstallPath")
	//      jdk := []string{jdkVersion, jdkPath}
	jdk := JDK{}
	if err := request.ReadEntity(&jdk); err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return

	}
	//
	InsertJdk(jdk.Version, jdk.Path)

	response.WriteHeaderAndEntity(http.StatusCreated, &jdk)

}

func deleteJdk(request *restful.Request, response *restful.Response) {

	jdkVersion := request.PathParameter("jdkVersion")

	if err := DeleteJdk(jdkVersion); err != nil {
		Info("fail to delete jdk:%v", err)
	}
	response.WriteHeader(http.StatusAccepted)

}
