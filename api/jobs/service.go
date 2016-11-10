package jobs

import (
	"github.com/emicklei/go-restful"
	//. "github.com/haveatry/She-Ra/api/response"
	. "github.com/haveatry/She-Ra/configdata"
	. "github.com/haveatry/She-Ra/utils"
)

func WebService(jobMng *JobManager) *restful.WebService {
	dc := jobMng
	return dc.WebService()
}

func Register(jobMng *JobManager, container *restful.Container, cors bool) {
	dc := jobMng
	dc.Register(container, cors)
}

func (d JobManager) Register(container *restful.Container, cors bool) {
	ws := d.WebService()

	// Cross Origin Resource Sharing filter
	if cors {
		corsRule := restful.CrossOriginResourceSharing{ExposeHeaders: []string{"Content-Type"}, CookiesAllowed: false, Container: container}
		ws.Filter(corsRule.Filter)
	}

	// Add webservice to container
	container.Add(ws)
}

func (d JobManager) WebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.Path("/She-Ra")
	ws.Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML)

	ws.Route(ws.GET("/jobs/get/{namespace}/{job-id}").To(d.readJob).
		// docs
		Doc("get a job config").
		Operation("readJob").
		Param(ws.PathParameter("namespace", "identifier of the namespace").DataType("string")).
		Param(ws.PathParameter("job-id", "identifier of the job").DataType("string")).
		Writes(Job{})) // on the response

	ws.Route(ws.GET("/jobs/getall").To(d.readAllJobs).
		// docs
		Doc("get job list").
		Operation("readAllJobs").
		Param(ws.PathParameter("namespace", "identifier of the namespace").DataType("string")).
		Param(ws.PathParameter("job-id", "identifier of the job").DataType("string")).
		Writes([]JobView{})) // on the response

	ws.Route(ws.POST("/jobs/create/{namespace}").To(d.createJob).
		// docs
		Doc("create a job").
		Operation("createJob").
		Param(ws.PathParameter("namespace", "identifier of the namespace").DataType("string")).
		Reads(Job{})) // from the request

	ws.Route(ws.POST("/jdk/create").To(createJdk).
		// docs
		Doc("create a jdk").
		Operation("createJdk").
		Reads(JDK{})) // from the request
	ws.Route(ws.PUT("/jobs/update").To(d.updateJob).
		// docs
		Doc("update a job").
		Operation("updateJob").
		Param(ws.PathParameter("namespace", "identifier of the namespace").DataType("string")).
		Reads(Job{})) // from the request

	ws.Route(ws.POST("/jobs/exec/{namespace}/{job-id}").To(d.execJob).
		// docs
		Doc("execute a job").
		Operation("execJob").
		Param(ws.PathParameter("namespace", "identifier of the namespace").DataType("string")).
		Param(ws.PathParameter("job-id", "identifier of the job").DataType("string")))

	ws.Route(ws.DELETE("/jobs/del/{namespace}/{job-id}").To(d.delJob).
		// docs
		Doc("delete a job").
		Operation("delJob").
		Param(ws.PathParameter("namespace", "identifier of the namespace").DataType("string")).
		Param(ws.PathParameter("job-id", "identifier of the job").DataType("string")))

	ws.Route(ws.GET("/jobs/get/{namespace}/{job-id}/executions").To(d.getAllJobExecutions).
		// docs
		Doc("get all job execution records").
		Operation("getAllJobExecutions").
		Param(ws.PathParameter("namespace", "identifier of the namespace").DataType("string")).
		Param(ws.PathParameter("job-id", "identifier of the job").DataType("string")).
		Reads([]ExecView{}))

	ws.Route(ws.GET("/jobs/get/{namespace}/{job-id}/{execution_id}").To(d.openJobExecution).
		// docs
		Doc("read a job execution record").
		Operation("openJobExecution").
		Param(ws.PathParameter("namespace", "identifier of the namespace").DataType("string")).
		Param(ws.PathParameter("job-id", "identifier of the job").DataType("string")).
		Param(ws.PathParameter("execution_id", "identifier of one job execution").DataType("int")).
		Reads(Execution{}))

	ws.Route(ws.PUT("/jobs/kill/{namespace}/{job-id}/{execution_id}").To(d.killJobExecution).
		// docs
		Doc("force stop a job execution").
		Operation("killJobExecution").
		Param(ws.PathParameter("namespace", "identifier of the namespace").DataType("string")).
		Param(ws.PathParameter("job-id", "identifier of the job").DataType("string")).
		Param(ws.PathParameter("execution_id", "identifier of one job execution").DataType("int")))

	ws.Route(ws.DELETE("/jobs/del/{namespace}/{job-id}/{execution_id}").To(d.delJobExecution).
		// docs
		Doc("delete a job execution record").
		Operation("delJobExecution").
		Param(ws.PathParameter("namespace", "identifier of the namespace").DataType("string")).
		Param(ws.PathParameter("job-id", "identifier of the job").DataType("string")).
		Param(ws.PathParameter("execution_id", "identifier of one job execution").DataType("int")))

	ws.Route(ws.POST("/jdk/del/{jdkVersion}").To(deleteJdk).
		// docs
		Doc("delete a jdk").
		Operation("deleteJdk").
		Param(ws.PathParameter("jdkVersion", "identifier of the jdk").DataType("string")))

	return ws
}
