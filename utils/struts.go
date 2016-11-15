package utils

// when some push to gitlab, hook send some msg to this server and ...
type Hook struct {
	Object_kind string `json: "object_kind"`
	Before string `json: "before"`
	After string `json: "after"`
	Ref string `json:"ref"`
	Checkout_sha string `json:"checkout_sha"`
	Message string `json:"message"`
	User_id int `json:"user_id"`
	User_name string `json: "user_name"`
	User_mail string `json: "user_email"`
	User_avatar string `json: "user_avatar"`
	Project_id int `json: "project_id"`
	Project HookProject `json: "project"`
	Commits []Commit `json: "commits"`
	Total_commits_count int `json: "total_commits_count"`
	Repository Repos  `json: "repository"`
}

type HookProject struct {
	Name string `json: "name"`
	Description string `json: "description"`
	Web_url string `json: "web_url"`
	Avatar_url string `json: "avatar_url"`
	Git_ssh_url string `json: "git_ssh_url"`
	Git_http_url string `json: "git_http_url"`
	Namespace string `json: "namespace"`
	Visibility_level int `json: "visibility_level"`
	Path_with_namespace string `json: "path_with_namespace"`
	Default_branch string `json: "default_branch"`
	Homepage string `json: "homepage"`
	Prourl string `json: "url"`
	Ssh_url string `json: "ssh_url"`
	http_url string `json: "http_url"`

}

type Commit struct {
	Id string `json: "id"`
	Message string `json: "message"`
	Timestamp string `json: "timestamp"`
	Url string `json: "url"`
	Author Au `json: "author"`
	Added []string `json: "added"`
	Modeify []string `json:"modified"`
	Removed []string `json:"removed"`
}

type Au struct {
	Auname string `json: "name"`
	Auemail string  `json: "email"`

}

type Repos struct {
	Repname string `json:"name"`
	Repurl string `json:"url"`
	Repdesc string `json: "description"`
	Rephomepage string `json: "homepage"`
	Repgithttpurl string `json: "git_http_url"`
	Repgitsshurl string `json: "git_ssh_url"`
	Repvisilevel string `json: "visibility_level"`
	
}

// get project from gitlab
type Project struct {
	Id int `json:"id"`
	Description string `json:"description`
	Default_branch string `json:"default_branch"`
	Tag_list []string `json:"tag_list"`
	Public bool `json:"public"`
	Archived bool `json:"archived"`
	Visibility_level int 	`json:"visibility_level"`
	Ssh_url_to_repo string `json:"ssh_url_to_repo"`
	Http_url_to_repo string `json:"http_url_to_repo"`
	Web_url string `json:"web_url"`
	Owner Own `json:"owner"`
	Name string `json:"name"`
	Name_with_namespace string `json:"name_with_namespace"`
	Path string `json:"path"`
	Path_with_namespace string `json:"path_with_namespace"`
	Issues_enabled bool `json:"issues_enabled"`
	Merge_requests_enabled bool 	`json:"merge_requests_enabled"`
	Wiki_enabled bool `json:"wiki_enabled"`
	Builds_enabled bool `json:"builds_enabled"`
	Snippets_enabled bool `json:"snippets_enabled"`
	Created_at string `json:"created_at"`
	Last_activity_at string `json:"last_activity_at"`
	Shared_runners_enabled bool `json:"shared_runners_enabled"`
	Creator_id int `json:"creator_id"`
	Namespace Nspace `json:"namespace"`
	
	Avatar_url string `json:"avatar_url"`
	Star_count int `json:"star_count"`
	Forks_count int `json:"forks_count"`
	Open_issues_count int `json:"open_issues_count"`
	Public_builds bool `json:"public_builds"`
	Permissions Perm `json:"permissions"`
}

type Own struct {
	    Name string `json:"name"`
            Username string `json:"username"`
            Id int `json:"id"`
            State string `json:"state"`
            Avatar_url string `json:"avatar_url"`
            Web_url string `json:"web_url"`
}

type Perm struct {
	//Project_access string `json:"project_access"`
	Project_access PAccess `json:"project_access"`
	Group_access string `json:"group_access"`
}

type PAccess struct {
	 Access_level int `json:"access_level"`
         Notification_level int  `json:"notification_level"`
}

type Nspace struct {
	Id int `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	Owner_id int `json:"owner_id"`
	Created_at string `json:"created_at"`
	Updated_at string `json:"updated_at"`
	Description string `json:"description"`
	Avatar Ava `json:"avatar"`
	Share_with_group_lock bool `json:"share_with_group_lock"`
	Visibility_level int `json:"visibility_level"`
}

type Ava struct {
	Url string `json:"url"`
}

// showlog
type Comlog struct { 
	Content string `json: "content"` 
	Seek int `json: "seek"`
	Flag bool `json: "flag"`
}

// git changes 

type Comgit struct { 
	Time string `json: "time"`
	Ns string `json:"namespace"`
	Job string `json:"job"`
	Flag bool `json: "flag"`
}

// git info

type GitInfo struct {
        Namespace string
        Proname string
	Type	string
}

// add hook
type Addhook struct{
        Id              string          `json: "id"`
        Url             string          `json: "url"`
        Push_events     bool            `json: "push_events"`
        Issues_events   bool            `json: "issues_events"`
        Merge_requests_events bool      `json: "merge_requests_events"`
        Tag_push_events bool            `json: "tag_push_events"`
        Note_events     bool            `json: "note_events"`
        Enable_ssl_verification bool    `json: "enable_ssl_verification"`
}
// response of adding hook
type Reshook struct{
        Id              int          `json: "id"`
        Url             string          `json: "url"`
	Created_at	string		`json: "created_at"`
	Project_id	int		`json: "project_id"`
        Push_events     bool            `json: "push_events"`
        Issues_events   bool            `json: "issues_events"`
        Merge_requests_events bool      `json: "merge_requests_events"`
        Tag_push_events bool            `json: "tag_push_events"`
        Note_events     bool            `json: "note_events"`
	Build_events	bool		`json: "build_events"`
        Enable_ssl_verification bool    `json: "enable_ssl_verification"`
}


