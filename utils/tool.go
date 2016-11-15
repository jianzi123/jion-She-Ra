package utils

import (
	"fmt"
	"time"
	"strings"
	"strconv"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"encoding/json"
	"html"
	"github.com/haveatry/She-Ra/configdata"
)

const (
	// get project id from git 
	GITBASEURL  = "http://code.bonc.com.cn/api/v3"
	GITLISTPRO  = "/projects/all?simple=yes"
	GITPRITOKEN = "54qcouoSZLxBHzJfoh6E"
	SSH = "ssh"
	HTTP = "http"
	// add project hook of git
	GITADDHOOKPRE = "/projects"
        GITADDHOOKEND = "/hooks"
	// add project hook of git; listen address	
	WEBHOOKPROTOCOL = "http://"
	WEBHOOKIPPORT = "192.168.56.101:8282"
	WEBHOOKLISTENROUTE = "/git"
	// delete hook
	GITDELHOOKPRE = "/projects"
	GITDELHOOKEND = "/hooks"
	// get hooks
	GITLISTHOOKPRE = "/projects"
	GITLISTHOOKEND = "/hooks"
)
// communicate with BCM
func NewComlog(c string, s int, f bool) (*Comlog, error) {
        log := &Comlog{
                Content: c,
                Seek:    s,
                Flag:    f,
        }
        return log, nil
}

func NewComgit(t string, ns string, j string, f bool) (*Comgit, error) {
        git := &Comgit{
                Time: t,
                Ns:   ns,
                Job:  j,
                Flag: f,
        }
        return git, nil
}
// get project id from git
func NewGitinfo(ns string, p string, t string) ( *GitInfo, error) {
        gitInfo := &GitInfo{
                Namespace: ns,
                Proname:   p,
                Type:      t,
        }
        return gitInfo, nil
}
// add hook
func NewHookData(id string, url string, push bool) (*Addhook, error){
        if id == "" || url == "" {
                return nil, errors.New("id or url is null.")
        }
        hook := &Addhook{
                Id: id,
                Url: url,
                Push_events: push,
                Issues_events: false,
                Merge_requests_events: false,
                Tag_push_events: false,
                Note_events: false,
                Enable_ssl_verification: false,
        }
        return  hook, nil

}
// list hooks and get hook info
func NewHook() (*Reshook, error) {
	h := &Reshook{
		Id: 0,
		Url: "",
		Created_at: "",
		Project_id: 0,
		Push_events: false,
		Issues_events: false,
		Merge_requests_events: false,
		Tag_push_events: false,
		Note_events: false,
		Build_events: false,
		Enable_ssl_verification: false,
	}
	return h, nil
}
// craete log file name

func CreateFname(id int) (string, error) {
	t := time.Now()
        sTime := t.Format("2006-01-02-15:04:05.000")
	suffix, err := FormN(id)
	if err != nil {
		Info("get time format failed: %v", err)
		return "", err
	}
        fName := fmt.Sprintf("%s_%s", sTime, suffix)
	return fName, nil	

}

func FindFname(fPath string, id string) (string ,error) {
	match, err := FormS(id)
	if err != nil	{
		return "", err
	}
	fName, err := MatchFname(fPath, match)
	if err != nil {
		return "", err
	}
	return fName, nil
}

func FormS(id string) (string, error){
	s := 5
        l := len(id)
        if l > s {
                Info("id is too long: %d", l)
                return "", errors.New("id is too long")
        }

        lack := s - l
        if lack == 0 {
                return id, nil
        }
        for ; lack > 0; lack-- {
                id = "0" + id
        }

        return id, nil

}
func FormN(id int) (string, error) {
	s := 5
	i := strconv.Itoa(id)
	l := len(i)
	if l > s {
		Info("id is too long: %d.", l)
		return "", errors.New("id is too long")
	}

	lack := s - l
	if lack == 0 {
		return i, nil
	}	
	for ; lack > 0; lack-- {
		i = "0" + i
	}
	
	return i, nil
}

func MatchFname(fPath string, match string) (string, error){ 
	var fName string
	dir_list, err := ioutil.ReadDir(fPath) 
	if err != nil { 
		Info("read dir error: %v", err) 
		return "", err
	} 
	for _, v := range dir_list { 
		fName = v.Name()
		if len(fName) < 6 {
			continue
		}else if strings.Contains(fName, match) == true{
			return fName, nil
		}
	}
	return "", errors.New("no match fileName")
}

// git
// get project id from git
func GetPid(git string) (int, error) {
        
	gitInfo, err := ParseGit(git)
        if err != nil {
                return 0, err
        }
	url := fmt.Sprintf("%s%s&search=%s&private_token=%s", GITBASEURL, GITLISTPRO, gitInfo.Proname, GITPRITOKEN )
	//Info("url: %s", url)
        //url := fmt.Sprintf("http://code.bonc.com.cn/api/v3/projects/all?simple=yes&search=%s&private_token=54qcouoSZLxBHzJfoh6E", gitInfo.Proname)
        resp, err := http.Get(url)
        if err != nil {
                Info("Http get url failed: %v.", err)
                return 0, err
        }

        defer resp.Body.Close()
        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
                Info("read data from resp.body: %v.", err)
                return 0, err
        }
        //fmt.Println(string(body))
        var p []Project
        err = json.Unmarshal(body, &p)
        if err != nil {
                Info("unmarshal body from git: %v.", err)
                return 0, err
        }
        var id int
        var flag bool = false
        fmt.Println(len(p))
        for _, item := range p {
                //if item.Sshurlrepo == "ssh://git@code.bonc.com.cn:10022/springcloud/guyi-region-7.git" {
                //if item.Sshurlrepo == "ssh://git@code.bonc.com.cn:10022/wangshuaijian/showLog.git" {
		if gitInfo.Type == SSH {  
                	if item.Ssh_url_to_repo == git && item.Namespace.Name == gitInfo.Namespace{
                        	id = item.Id
                        	flag = true
                        	break
                	}
		}else if gitInfo.Type == HTTP {
			if item.Http_url_to_repo == git && item.Namespace.Name == gitInfo.Namespace{
				id = item.Id
				flag = true
				break
			}
		}else if item.Web_url == git && item.Namespace.Name == gitInfo.Namespace{
			id = item.Id
			flag = true
			break
		}
                //fmt.Println("id: ", item.Id, "; ")
        }
        if flag != true {
                Info("can not find git.")
		return 0, errors.New("cannot find git id.")
        }else{
                //Info("find git, id: %d", id)
		return id, nil
        }
}


func ParseGit(git string) (*GitInfo, error) {
        if len(git) <= 0 {
                fmt.Println("git is empty")
                return nil, errors.New("git is empty.")
        }
        var name, pro string

        info := strings.Split(git, "/")
        l := len(info)
        if l < 5 {
                fmt.Println("git is not correct format." + git)
                return nil, errors.New("git is not correct format." + git)
        }
        if strings.Contains(info[l - 1], ".git") != true {
                fmt.Println("git is not correct format." + git)
                return nil, errors.New("git is not correct format." + git)
        }
        name = info[l - 2]
        pro  = info[l - 1]
        proInfo := strings.Split(pro, ".git")
        if len(proInfo) < 2 {
                fmt.Println("git is not correct format." + git)
                return nil, errors.New("git is not correct format." + git)

        }
	var t string
	if strings.Contains(git, "ssh://") == true {
		t = SSH
	}else if strings.Contains(git, "http://") == true {
		t = HTTP
	}
        pro = proInfo[0]
	/*
        gitInfo := &GitInfo{
                Namespace: name,
                Proname:   pro,
		Type:	   t,
        }
	*/
	gitInfo, _ := NewGitinfo(name, pro, t)
        return gitInfo, nil
}

// add git hook
func AddFormHook(h *Addhook, pid int) (*Reshook, error){
        v := url.Values{}
        v.Set("id", h.Id)
        v.Set("url", h.Url)
        v.Set("push_events", BoolToString(h.Push_events))
	v.Set("issues_events", BoolToString(h.Issues_events))
	v.Set("merge_requests_events", BoolToString(h.Merge_requests_events))
	v.Set("tag_push_events", BoolToString(h.Tag_push_events))
	v.Set("note_events", BoolToString(h.Note_events))
	v.Set("enable_ssl_verification", BoolToString(h.Enable_ssl_verification))
	Info("%v.", v)
        body := ioutil.NopCloser(strings.NewReader(v.Encode())) //把form数据编下码
        client := &http.Client{}
	url := fmt.Sprintf("%s%s/%d%s?private_token=%s", GITBASEURL, GITADDHOOKPRE, pid, GITADDHOOKEND, GITPRITOKEN)
        //Info("url: %s.", url)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		Info("new resquest failed: %v.", err)
		return nil, err
	}
        req.Header.Set("Content-Type", "application/x-www-form-urlencoded; param=value") //这个一定要加，不加form的值post不过去，被坑了>两小时

        resp, err := client.Do(req) //发送
        defer resp.Body.Close()     //一定要关闭resp.Body
	if err != nil {
		Info("send to git failed: %v.", err)
		return nil, err
	}
        data, err := ioutil.ReadAll(resp.Body)
        if err != nil {
		Info("need to make sure, whether add hook: %v.", err)
		return nil, err
	}

	var r Reshook
        err = json.Unmarshal(data, &r)
        if err != nil {
                Info("unmarshal body from git: %s; %v.", string(data), err)
                return nil, err
        }
	Info("%s; %v", string(data), r)	
	return &r, nil 
}

func Dealhook(g string) (int, error) {
                id, err := GetPid(g)
                if err != nil {
			return 0, err
                }else{
			hurl := fmt.Sprintf("%s%s%s", WEBHOOKPROTOCOL, WEBHOOKIPPORT, WEBHOOKLISTENROUTE)
			Info("%s.", hurl)
                        h, err := NewHookData(strconv.Itoa(id), hurl, true)
                        r, err := AddFormHook(h, id)
			if err != nil {
				Info("add webhook failed.err: %v.", err)
				return 0, err
			}
			hId := r.Id
			return hId, nil
                } 

}

func DealAddhook(pid string) (int , error) {
                        hurl := fmt.Sprintf("%s%s%s", WEBHOOKPROTOCOL, WEBHOOKIPPORT, WEBHOOKLISTENROUTE)
                        //Info("%s.", hurl)
                        h, err := NewHookData(pid, hurl, true)
			id, err := strconv.Atoi(pid)
                        r, err := AddFormHook(h, id)
                        if err != nil {
                                Info("add webhook failed.err: %v.", err)
                                return 0, err
                        }
                        hId := r.Id
                        return hId, nil
}

func BoolToString(f bool) string {
	if f == true {
		return "1"
	}else{
		return "0"
	}
}
// delete webhook
func Delhook(g string, hid string) (error) {
	id, err := GetPid(g)
	if err != nil {
		return err
	}else{
		err := DelhookEnd(strconv.Itoa(id), hid)
		if err != nil {
			Info("Del hook failed: %v", err)
			return err
		}
	}
	return nil
}

func DelhookEnd(pid string, hid string) (error) {
	url := fmt.Sprintf("%s%s/%s%s/%s?private_token=%s", GITBASEURL, GITDELHOOKPRE, pid, GITDELHOOKEND, hid, GITPRITOKEN)
	//Info("url: %s.", url)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		Info("new request failed: %v.", err)
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	defer resp.Body.Close()     //一定要关闭resp.Body
        if err != nil {
                Info("delete failed: %v.", err)
                return err
        }
        data, err := ioutil.ReadAll(resp.Body)
        if err != nil {
                Info("need to make sure, whether add hook: %v.", err)
                return err
        }
	Info("show response : %s", string(data))
	return nil
}

func GetHooks(pid string) ([]Reshook, error) {
	if len(pid) == 0 {
		return nil, errors.New("pid is empty")
	}
	url := fmt.Sprintf("%s%s/%s%s?private_token=%s", GITBASEURL, GITLISTHOOKPRE, pid, GITLISTHOOKEND, GITPRITOKEN )
        //Info("url: %s.", url)
	resp, err := http.Get(url)
        if err != nil {
                Info("Http get url failed: %v.", err)
                return nil, err
        }

        defer resp.Body.Close()
        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
                Info("read data from resp.body: %v.", err)
                return nil, err
        }
	fmt.Println(string(body))
        var p []Reshook
        err = json.Unmarshal(body, &p)
        if err != nil {
                Info("unmarshal body from git: %v.", err)
                return nil, err
        }
	Info("%v", p)
	return p, nil
}

func GethookInfo(pid string, url string) (string, error) {
	hooks, err := GetHooks(pid)
	//Info("%s", hooks)
	if err != nil {
		Info("%s", hooks)
		return "", err
	}
	if len(hooks) == 0 {
		//Info("%s", hooks)
		return "", nil
	}
	for _, item := range hooks {
		if item.Url == url {
			return strconv.Itoa(item.Id), nil
		}
	}	
	Info("%s", hooks)
	return "", nil
}

func RecvGit(w http.ResponseWriter, r *http.Request){
	r.ParseForm()
        fmt.Fprintf(w, "receive %s \n", html.EscapeString(r.URL.Path[1:]))
        fmt.Fprintf(w, "method %s  \n", r.Method)
        if r.Method == "POST" {
                result, err := ioutil.ReadAll(r.Body)
                if err != nil {
                        fmt.Println(err)
                        return
                }

                defer r.Body.Close()

                fmt.Println(string(result))
                var h Hook
                err = json.Unmarshal(result, &h)
                if err != nil {
                        fmt.Println("----", err)
                        return
                }
                fmt.Println(h)
                fmt.Println(h.Object_kind, ";project id: ", h.Project_id, "git: ", h.Project.Ssh_url, h.Project.Name, h.Project.Namespace )
		t    := time.Now().Unix()
		ssh  := h.Project.Git_ssh_url
		http := h.Project.Git_http_url
		b    := h.Project.Default_branch
		UpdateJobViewPushTime(ssh, http, b, t)
		return
        }
}

func UpdateJobViewPushTime(ssh string, http string, branch string , pushTime int64) {
	var j string
	if ssh != "" && http != "" {
		j = fmt.Sprintf(" git = %s OR %s ", ssh, http)
	}else if ssh != "" {
		j = fmt.Sprintf(" git = %s ", ssh)
	}else if http != "" {
		j = fmt.Sprintf(" git = %s ", http)
	}else{
		Info("ssh and http is empty.")
		return
	}
	sql := fmt.Sprintf("update jobView set hookTime = %d where %s AND branch = %s", pushTime, j, branch)
        if stmt, err := Database.Prepare("update jobView set hookTime = ? where ? AND branch = ?"); err != nil {
                Info("Failed to prepare update sql; sql: %s; err: %v\n", sql, err)
        } else if _, err := stmt.Exec(pushTime, j, branch); err != nil {
                Info("Failed to update data git PushTime; sql: %s; err: %v\n", sql, err)
        }
	return
}

func GetGitExist(g string, b string) (int,  error) {
        rows, err := Database.Query("SELECT hookid FROM jobView WHERE git = ? AND branch = ?", g, b)
        if err != nil {
                Info("Failed to get git info from db: %v.", err)
                return 0,  err
        }

        var n, value int
	value = -1
        for rows.Next() {
		n = n + 1
        	if err := rows.Scan(&value); err != nil {
                	Info("scan err: %v.", err)
        	}
	}
	if n > 0 {
		return value, nil
	}else{
		return 0, nil
	}
}

func GetGitInfo(ns string, j string) (string, string, string, error) {
	sql := fmt.Sprintf("SELECT hookId, git, branch FROM jobView WHERE namespace = '%s' AND 'jobId' = %s", ns, j)
        rows, err := Database.Query("SELECT hookId, git, branch FROM jobView WHERE namespace = ? AND jobId = ?", ns, j)
        if err != nil {
                Info("Failed to get git info from db; sql: %s; err: %v.", sql, err)
                return "", "", "", err 
        }       
        var hId, g, b string
        for rows.Next() {
                if err := rows.Scan(&hId, &g, &j); err != nil {
                        Info("Scan err; sql: %s; err: %v.", sql, err)
			return "", "", "", err
                }       
        }       
	return hId, g, b, nil

}

func GetGitToDel(g string, b string, ns string, j string) (int64, error) {
	sql := fmt.Sprintf("SELECT hookid FROM jobView WHERE git = %s AND branch = %s AND namespace != %s AND jobId != %s", g, b, ns, j)
        rows, err := Database.Query("SELECT hookid FROM jobView WHERE git = ? AND branch = ? AND namespace != ? AND jobId != ?", g, b, ns, j)
        if err != nil {
                Info("Failed to get git info from db; sql: %s; err: %v.", sql, err)
                return -1, err
        }
	var n int
	var value int64
	for rows.Next() {
		n = n + 1
                if err := rows.Scan(&value); err != nil {
                        Info("Scan err: %v.", err)
			return -1, err
              	}
        }
	if n > 0 {
		return value, nil
	}else{
		return -1, nil
	}
	
}

func GetGitTime(ns string, j string) ([]int64, error) {
	sql := fmt.Sprintf("SELECT hookTime FROM jobView WHERE namespace = %s AND jobId = %s;", ns, j)
        rows, err := Database.Query("SELECT hookTime FROM jobView WHERE namespace = ? AND jobId = ?", ns, j)
        var g []int64
        if err != nil {
                Info("Failed to get gitTime; sql: %s; err: %v.", sql, err)
                return nil, err
        }
        var gTime int64
        for rows.Next() {
                if err := rows.Scan(&gTime); err != nil {
                        Info("Scan faided; sql success: %s; err: %v.", sql, err)
                }
                g = append(g, gTime)
        }
        return g, nil
}

func UpdateJobViewHook(namespace, jobId, url, branch string, id int64) error {
	sql := fmt.Sprintf("update jobView set git = %s, branch = %s, hookid = %d where namespace = %s and jobId = %s;", url, branch, id, namespace, jobId)
        if stmt, err := Database.Prepare("update jobView set git = ?, branch = ?, hookid = ? where namespace = ? and jobId = ?"); err != nil {
                Info("Failed to prepare update sql; sql: %s; %v\n", sql, err)
		return err
        } else if _, err := stmt.Exec(url, branch, id, namespace, jobId); err != nil {
                Info("Failed to update data about git; sql: %s; %v\n", sql, err)
		return err
        }
	return nil
}
// update job
func UpdateHook(job configdata.Job, key Key) {
	
        // add git hook wsj
        var gUrl, gBranch string
        var id int = 0
        if codeManager := job.GetCodeManager(); codeManager != nil && codeManager.GitConfig != nil {
                gUrl = codeManager.GitConfig.Repo.Url
                gBranch = codeManager.GitConfig.Branch
		// 从git获取url对应的hookid,如果没有
                gId, err := GetPid(gUrl)// get project id in git
                if err == nil {
                        hookid, err := GethookInfo(strconv.Itoa(gId), gUrl)
                        if err == nil  && hookid != "" {
                                id, err = strconv.Atoi(hookid)
                        }else if err == nil && hookid == "" {
				id, err = DealAddhook(strconv.Itoa(gId))
			}else{
				Info("Get hook inof failed.%v.", err)
			}       
                }else {
			// judge project exist
                        n, err := GetGitExist(gUrl, gBranch)
                        if err != nil {
                                Info("Get Git hook id failed: %v.", err)
                        }else if n == 0 {
                                id, err = Dealhook(gUrl)
                        }else{
                                id = n
                        }
		}
        }else{  
                Info("get code info from git config and data is empy.") 
        }       
        if err := UpdateJobViewHook(key.Ns, key.Id, gUrl, gBranch, int64(id)); err != nil {
                Info("Update job view failed: namespace:%s, job id: %s, url: %s, branch: %s, id:%d, %v. ", key.Ns, key.Id, gUrl, gBranch, id,  err)
                return  
        }       
}
