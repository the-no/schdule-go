package manager

import (
	"fmt"
	"github.com/go-martini/martini"
	"github.com/martini-contrib/binding"
	"github.com/martini-contrib/render"
	"github.com/martini-contrib/web"
	"gitlab.51idc.com/hds/scheduling/schedule"
	"log"
	"net/http"
	"strconv"
	"time"
)

var (
	g *schedule.GlobalConfigStruct
)

//初始化并启动web服务
func StartManager(sl *schedule.ScheduleManager) { // {{{
	g = sl.Global
	m := martini.Classic()
	m.Use(Logger)
	m.Use(martini.Static("web/public"))
	m.Use(web.ContextWithCookieSecret(""))
	m.Use(render.Renderer(render.Options{
		Directory:       "web/templates",            // Specify what path to load the templates from.
		Extensions:      []string{".tmpl", ".html"}, // Specify extensions to load for templates.
		Delims:          render.Delims{"{[{", "}]}"},
		Charset:         "UTF-8",     // Sets encoding for json and html content-types. Default is "UTF-8".
		IndentJSON:      true,        // Output human readable JSON
		IndentXML:       true,        // Output human readable XML
		HTMLContentType: "text/html", // Output XHTML content type instead of default "text/html"
	}))

	m.Map(sl)
	controller(m)

	g.L.Println("Web manager is running in ", g.ManagerPort)
	err := http.ListenAndServe(g.ManagerPort, m)
	if err != nil {
		log.Fatal("Fail to start server: %v", err)
	}
} // }}}

//controller转发规则设置
func controller(m *martini.ClassicMartini) { // {{{
	m.Get("/", func(r render.Render) {
		r.HTML(200, "index", nil)
	})

	m.Group("/tasks", func(r martini.Router) {
		//Task部分
		r.Get("", GetScheduleById)
		r.Get("/:id", GetTask)
		r.Post("", binding.Bind(schedule.Task{}), AddTask)
		r.Put("/:id", binding.Bind(schedule.Task{}), UpdateTask)
		r.Delete("/:id", DeleteTask)

		//TaskRelation部分
		r.Post("/:id/reltask/:relid", AddRelTask)
		r.Delete("/:id/reltask/:relid", DeleteRelTask)

		r.Post("/dotasks/:id", DoTask)
	})

} // }}}

//返回当前的调度列表
func GetSchedules(r render.Render, Ss *schedule.ScheduleManager) { // {{{
	r.JSON(200, Ss.ScheduleList)
	return
} // }}}

//根据参数中的Id，返回对应的Schedule信息
func GetScheduleById(params martini.Params, r render.Render, Ss *schedule.ScheduleManager) { // {{{
	i, _ := params["id"]
	id, _ := strconv.Atoi(i)
	for _, s := range Ss.ScheduleList {
		if s.Id == int64(id) {
			r.JSON(200, s)
			return
		}
	}
	r.JSON(500, fmt.Sprintf("[GetScheduleById] not found Schedule [%s]", params["id"]))
	return

} // }}}

//添加Schedule
func AddSchedule(params martini.Params, r render.Render, Ss *schedule.ScheduleManager, scd schedule.Schedule) { // {{{
	if scd.Name == "" {
		e := fmt.Sprintf("[AddSchedule] Schedule name is required")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}

	err := Ss.AddSchedule(&scd)
	if err != nil {
		e := fmt.Sprintf("[AddSchedule] add schedule error %s.", err.Error())
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}

	r.JSON(200, scd)
	return
} // }}}

//updateSchedule获取客户端发送的Schedule信息，并调用Schedule的Update方法将其
//持久化并更新至Schedule中。
//成功返回更新后的Schedule信息
func UpdateSchedule(params martini.Params, r render.Render, Ss *schedule.ScheduleManager, scd schedule.Schedule) { // {{{
	if scd.Name == "" {
		e := fmt.Sprintf("[UpdateSchedule] Schedule name is required")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}
	fmt.Println(scd)
	if s := Ss.GetScheduleById(int64(scd.Id)); s != nil {
		s.Name, s.Desc, s.Cyc = scd.Name, scd.Desc, scd.Cyc
		s.ModifyTime, s.ModifyUserId = time.Now(), scd.ModifyUserId
		if err := s.UpdateSchedule(); err != nil {
			e := fmt.Sprintf("[UpdateSchedule] update schedule error %s.", err.Error())
			g.L.Warningln(e)
			r.JSON(500, e)
			return
		} else {
			r.JSON(200, s)
		}
	} else {
		e := fmt.Sprintf("[UpdateSchedule] schedule not found.")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}
} // }}}

//调用Schedule的DeleteJob方法删除作业
func DeleteJob(params martini.Params, r render.Render, Ss *schedule.ScheduleManager) { // {{{

	sid, sidok := params["sid"]
	id, idok := params["id"]

	if !sidok || !idok {
		e := fmt.Sprintf("[DeleteJob] sid or id not null.")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}

	ssid, _ := strconv.Atoi(sid)
	iid, _ := strconv.Atoi(id)

	if s := Ss.GetScheduleById(int64(ssid)); s != nil {
		if err := s.DeleteJob(int64(iid)); err != nil {
			e := fmt.Sprintf("[DeleteJob] delete job error %s.", err.Error())
			g.L.Warningln(e)
			r.JSON(500, e)
			return
		} else {
			e := fmt.Sprintf("[DeleteJob] delete job success.")
			r.JSON(204, e)
		}

	}

} // }}}

//addJob获取客户端发送的Job信息，并调用Schedule的AddJob方法将其
//持久化并添加至Schedule中。
//成功返回添加好的Job信息
//错误返回err信息
func AddJob(r render.Render, Ss *schedule.ScheduleManager, job schedule.Job) { // {{{
	if job.Name == "" {
		e := fmt.Sprintf("[AddJob] Job name is required")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}
	if s := Ss.GetScheduleById(int64(job.ScheduleId)); s != nil {
		job.ScheduleCyc = s.Cyc
		job.CreateUserId = 1
		job.ModifyUserId = 1
		job.CreateTime = NowTimePtr()
		job.ModifyTime = NowTimePtr()
		if err := s.AddJob(&job); err != nil {
			e := fmt.Sprintf("[AddJob] add job error %s.", err.Error())
			g.L.Warningln(e)
			r.JSON(500, e)
			return
		} else {
			r.JSON(200, job)
		}
	} else {
		e := fmt.Sprintf("[AddJob] schedule not found.")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}
} // }}}

//updateJob获取客户端发送的Job信息，并调用Schedule的UpdateJob方法将其
//持久化并更新至Schedule中。
//成功返回更新后的Job信息
func UpdateJob(r render.Render, Ss *schedule.ScheduleManager, job schedule.Job) { // {{{
	if job.Name == "" {
		e := fmt.Sprintf("[UpdateJob] Job name is required")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}
	if s := Ss.GetScheduleById(int64(job.ScheduleId)); s != nil {
		if err := s.UpdateJob(&job); err != nil {
			e := fmt.Sprintf("[UpdateJob] update job error %s.", err.Error())
			g.L.Warningln(e)
			r.JSON(500, e)
			return
		} else {
			r.JSON(200, job)
		}
	} else {
		e := fmt.Sprintf("[UpdateJob] schedule not found.")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}

} // }}}

func GetTask(params martini.Params, r render.Render, Ss *schedule.ScheduleManager) { // {{{
	sid, _ := strconv.Atoi(params["sid"])
	id, _ := strconv.Atoi(params["id"])

	if id == 0 {
		e := fmt.Sprintf("[GetTask Task] id is required")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}

	if s := Ss.GetScheduleById(int64(sid)); s != nil {
		t := s.GetTaskById(int64(id))
		r.JSON(200, t)
	} else {
		e := fmt.Sprintf("[GetTask Task] GetTask task error Not Found Schedule[%d].", sid)
		g.L.Warningln(e)
		r.JSON(500, e)
	}
}

//addTask获取客户端发送的Task信息，调用Task的AddTask方法持久化。
//成功后根据其中的JobId找到对应Job将其添加
//成功返回添加好的Job信息
//错误返回err信息
func AddTask(params martini.Params, r render.Render, Ss *schedule.ScheduleManager, task schedule.Task) { // {{{
	sid, _ := params["sid"]
	ssid, _ := strconv.Atoi(sid)
	if task.Name == "" {
		e := fmt.Sprintf("[AddTask] name is required")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}

	t := &task
	//t.TaskType = 1
	t.CreateUserId = 1
	t.ModifyUserId = 1
	t.CreateTime = NowTimePtr()
	t.ModifyTime = NowTimePtr()

	if err := t.AddTask(); err != nil {
		if err != nil {
			e := fmt.Sprintf("[AddTask] add task error %s.", err.Error())
			g.L.Warningln(e)
			r.JSON(500, e)
			return
		}
	}

	if s := Ss.GetScheduleById(int64(ssid)); s != nil {
		s.UpdateTask(t)
	}
	r.JSON(200, task)
} // }}}

func DoTask(params martini.Params, r render.Render, Ss *schedule.ScheduleManager) {
	sid, _ := strconv.Atoi(params["sid"])
	id, _ := strconv.Atoi(params["id"])

	if id == 0 {
		e := fmt.Sprintf("[DoTask Task] id is required")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}
	if s := Ss.GetScheduleById(int64(sid)); s != nil {
		t := s.GetTaskById(int64(id))
		r.JSON(200, t)
		s.DoTask(t)
	} else {
		e := fmt.Sprintf("[DoTask Task] DoTask task error Not Found Schedule[%d].", sid)
		g.L.Warningln(e)
		r.JSON(500, e)
	}
}

//deleteTask从调度结构中删除指定的Task，并持久化。
func DeleteTask(params martini.Params, r render.Render, Ss *schedule.ScheduleManager) { // {{{
	sid, _ := strconv.Atoi(params["sid"])
	id, _ := strconv.Atoi(params["id"])

	if id == 0 {
		e := fmt.Sprintf("[Delete Task] id is required")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}

	if s := Ss.GetScheduleById(int64(sid)); s != nil {
		t := s.GetTaskById(int64(id))
		if err := t.Delete(); err != nil {
			e := fmt.Sprintf("\n[s.DeleteTask] schedule [%d] Delete error %s.", err.Error())
			r.JSON(500, e)
			return
		}
		r.JSON(200, t)
		s.UpdateTask(t)
	} else {
		e := fmt.Sprintf("[Delete Task] delete task error Not Found Schedule[%d].", sid)
		g.L.Warningln(e)
		r.JSON(500, e)
	}

} // }}}

//updateTask获取客户端发送的Task信息，并调用Job的UpdateTask方法将其
//持久化并更新至Job中。
//成功返回更新后的Task信息
func UpdateTask(params martini.Params, r render.Render, Ss *schedule.ScheduleManager, task schedule.Task) { // {{{
	//var err error
	sid, _ := params["sid"]
	ssid, _ := strconv.Atoi(sid)
	id, _ := strconv.Atoi(params["id"])

	t := &task
	if t.Name == "" {
		e := fmt.Sprintf("[UpdateTask] task name is required")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}

	if s := Ss.GetScheduleById(int64(ssid)); s != nil {
		t := s.GetTaskById(int64(id))
		t.Name, t.Desc, t.Address = task.Name, task.Desc, task.Address
		t.TaskType, t.TaskCyc, t.StartSecond = task.TaskType, task.TaskCyc, task.StartSecond
		t.Cmd, t.TimeOut = task.Cmd, task.TimeOut
		t.Cronstr, t.Retry, t.Concurrent = task.Cronstr, task.Retry, task.Concurrent
		t.Attr, t.ModifyUserId, t.ModifyTime = task.Attr, task.ModifyUserId, NowTimePtr()
		if err := t.UpdateTask(); err != nil {
			e := fmt.Sprintf("\n[UpdateTask] UpdateTask error %s.", err.Error())
			r.JSON(500, e)
			g.L.Warningln(e)
			return
		} else {
			if s := Ss.GetScheduleById(int64(ssid)); s != nil {
				s.UpdateTask(t)
			}
			r.JSON(200, task)
		}
	} else {
		e := fmt.Sprintf("[Delete Task] delete task error Not Found Schedule[%d].", ssid)
		g.L.Warningln(e)
		r.JSON(500, e)
	}

	return
} // }}}

func GetJobsForSchedule(params martini.Params, r render.Render, res http.ResponseWriter, Ss *schedule.ScheduleManager) { // {{{

	sid, sidok := params["sid"]
	if !sidok {
		e := fmt.Sprintf("[GetJobsForSchedule] sid is required")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}

	ssid, _ := strconv.Atoi(sid)
	if s := Ss.GetScheduleById(int64(ssid)); s != nil {
		r.JSON(200, s.Jobs)
	} else {
		e := fmt.Sprintf("[GetJobsForSchedule] schedule not found.")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}
	return
} // }}}

func DeleteSchedule(params martini.Params, ctx *web.Context, r render.Render, Ss *schedule.ScheduleManager) { // {{{
	id, _ := strconv.Atoi(params["id"])

	if id == 0 {
		e := fmt.Sprintf("[DeleteSchedule] id is required")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}

	if err := Ss.DeleteSchedule(int64(id)); err != nil {
		e := fmt.Sprintf("[DeleteSchedule] delete schedule error %s.", err.Error())
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}
	r.JSON(200, nil)

} // }}}

//addRelTask根据Url参数获取到要添加的Task关系
func AddRelTask(params martini.Params, ctx *web.Context, r render.Render, Ss *schedule.ScheduleManager) { // {{{
	sid, _ := strconv.Atoi(params["sid"])
	//jid, _ := strconv.Atoi(params["jid"])
	id, _ := strconv.Atoi(params["id"])
	relid, _ := strconv.Atoi(params["relid"])

	if id == 0 || relid == 0 {
		e := fmt.Sprintf("[AddRelTask] [id relid] is required")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}

	if s := Ss.GetScheduleById(int64(sid)); s != nil {
		t := s.GetTaskById(int64(id))
		rt := s.GetTaskById(int64(relid))

		if t == nil || rt == nil {
			e := fmt.Sprintf("[AddRelTask] task or reltask is required")
			g.L.Warningln(e)
			r.JSON(500, e)
			return
		}

		err := t.AddRelTask(rt)
		if err != nil {
			e := fmt.Sprintf("[AddRelTask] add task is error %s.", err.Error())
			g.L.Warningln(e)
			r.JSON(500, e)
			return
		}
		//	if s := Ss.GetScheduleById(int64(sid)); s != nil {
		s.UpdateTask(t)
		//	}
		r.JSON(200, t)
	}

} // }}}

func DeleteRelTask(params martini.Params, ctx *web.Context, r render.Render, Ss *schedule.ScheduleManager) { // {{{
	sid, _ := strconv.Atoi(params["sid"])
	//jid, _ := strconv.Atoi(params["jid"])
	id, _ := strconv.Atoi(params["id"])
	relid, _ := strconv.Atoi(params["relid"])

	if id == 0 || relid == 0 {
		e := fmt.Sprintf("[DeleteRelTask] [id relid] is required")
		g.L.Warningln(e)
		r.JSON(500, e)
		return
	}

	if s := Ss.GetScheduleById(int64(sid)); s != nil {
		t := s.GetTaskById(int64(id))

		if t == nil {
			e := fmt.Sprintf("[DeleteRelTask] task is required")
			g.L.Warningln(e)
			r.JSON(500, e)
			return
		}

		err := t.DeleteRelTask(int64(relid))
		if err != nil {
			e := fmt.Sprintf("[DeleteRelTask] delete task is error %s.", err.Error())
			g.L.Warningln(e)
			r.JSON(500, e)
			return
		}
		//if s := Ss.GetScheduleById(int64(sid)); s != nil {
		s.UpdateTask(t)
		//}
		r.JSON(200, t)
	}

} // }}}

func Logger() martini.Handler { // {{{
	return func(res http.ResponseWriter, req *http.Request, ctx martini.Context, log *log.Logger) {

		start := time.Now()
		log.Printf("Started %s %s", req.Method, req.URL.Path)

		rw := res.(martini.ResponseWriter)
		ctx.Next()

		content := fmt.Sprintf("Completed %v %s in %v", rw.Status(), http.StatusText(rw.Status()), time.Since(start))
		switch rw.Status() {
		case 200:
			content = fmt.Sprintf("\033[1;32m%s\033[0m", content)
		case 304:
			content = fmt.Sprintf("\033[1;33m%s\033[0m", content)
		case 404:
			content = fmt.Sprintf("\033[1;31m%s\033[0m", content)
		case 500:
			content = fmt.Sprintf("\033[1;36m%s\033[0m", content)
		}
		log.Println(content)
	}
} // }}}
func NowTimePtr() *time.Time {
	t := time.Now()
	return &t
}
