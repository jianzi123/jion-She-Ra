package main

import (
	"flag"
	"github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/swagger"
	"github.com/fvbock/endless"
	"github.com/haveatry/She-Ra/api/jobs"
	"github.com/haveatry/She-Ra/utils"
	"github.com/magiconair/properties"
	"golang.org/x/net/websocket"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
)

var (
	props          *properties.Properties
	propertiesFile = flag.String("config", "she-ra.properties", "the configuration file")

	SwaggerPath string
	SheRaIcon   string
)

func handler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("WORLD!"))

}

func preSigUsr1() {
	log.Println("pre SIGUSR1")

}

func postSigUsr1() {
	log.Println("post SIGUSR1")

}

func main() {
	flag.Parse()

	// Load configurations from a file
	utils.Info("loading configuration from [%s]", *propertiesFile)
	var err error
	var jobMng *jobs.JobManager
	if props, err = properties.LoadFile(*propertiesFile, properties.UTF8); err != nil {
		log.Fatalf("[She-Ra][error] Unable to read properties:%v\n", err)
	}

	// Swagger configuration
	SwaggerPath = props.GetString("swagger.path", "")
	SheRaIcon = filepath.Join(SwaggerPath, "images/jion.ico")

	//set the log format and its destination file
	logFilePath := props.MustGet("log.path")
	if err = os.MkdirAll(logFilePath, 0755); err != nil {
		utils.Info("failed to create log file dir\n")
		os.Exit(1)

	}

	logFileName := logFilePath + "She-Ra.log"
	logFile, logErr := os.OpenFile(logFileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if logErr != nil {
		utils.Info("Fail to find %s\n", logFileName)
		os.Exit(1)

	} else {
		utils.Info("put log to %s\n", logFileName)
	}
	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// init workspace and database
	utils.Init(props)

	// New Job Manager
	if jobMng, err = jobs.NewJobManager(); err != nil {
		log.Fatalf("[She-Ra][error] failed to create JobManager:%v\n", err)
	}

	// accept and respond in JSON unless told otherwise
	restful.DefaultRequestContentType(restful.MIME_JSON)
	restful.DefaultResponseContentType(restful.MIME_JSON)

	// faster router
	restful.DefaultContainer.Router(restful.CurlyRouter{})
	// no need to access body more than once
	restful.SetCacheReadEntity(false)

	// API Cross-origin requests
	apiCors := props.GetBool("http.server.cors", false)

	//Register API
	jobs.Register(jobMng, restful.DefaultContainer, apiCors)

	addr := props.MustGet("http.server.host") + ":" + props.MustGet("http.server.port")
	basePath := "http://" + addr

	// Register Swagger UI
	swagger.InstallSwaggerService(swagger.Config{
		WebServices:     restful.RegisteredWebServices(),
		WebServicesUrl:  basePath,
		ApiPath:         "/apidocs.json",
		SwaggerPath:     SwaggerPath,
		SwaggerFilePath: props.GetString("swagger.file.path", ""),
	})

	log.Print("basePath: ", basePath, "SwaggerPath: ", SwaggerPath, "SheRaIcon: ", SheRaIcon)
	// If swagger is not on `/` redirect to it
	if SwaggerPath != "/" {
		http.HandleFunc("/", index)
	}

	// Serve favicon.ico
	http.HandleFunc("/favion.ico", icon)
	// log router
	http.HandleFunc("/client", utils.Client)
	http.Handle("/echoLog", websocket.Handler(jobMng.Log))

	utils.Info("ready to serve on %s", basePath)
	srv := endless.NewServer(addr, nil)
	srv.SignalHooks[endless.PRE_SIGNAL][syscall.SIGUSR1] = append(
		srv.SignalHooks[endless.PRE_SIGNAL][syscall.SIGUSR1],
		preSigUsr1,
	)
	srv.SignalHooks[endless.POST_SIGNAL][syscall.SIGUSR1] = append(
		srv.SignalHooks[endless.POST_SIGNAL][syscall.SIGUSR1],
		postSigUsr1,
	)

	log.Fatal(srv.ListenAndServe())
}

// If swagger is not on `/` redirect to it
func index(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, SwaggerPath, http.StatusMovedPermanently)
}

func icon(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, SheRaIcon, http.StatusMovedPermanently)
}
