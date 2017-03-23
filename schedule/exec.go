package schedule

import (
	"bytes"
	"errors"
	"fmt"
	"net/rpc"
	"runtime/debug"
	"sync"
	"time"
)

//根据传入的Schedule参数来构建一个调度的执行结构，并返回。
func ExecScheduleWarper(s *Schedule) *ExecSchedule { // {{{
	return &ExecSchedule{
		batchId:  fmt.Sprintf("%s %d", time.Now().Local().Format("2006-01-02 15:04:05.000000"), s.Id), //批次ID
		schedule: s,
		execType: 1,
		jobCnt:   s.JobCnt,
		//taskCnt:      s.TaskCnt,
		execTasks:    make(map[int64]*ExecTask), //设置任务列表
		execTaskChan: make(chan *ExecTask),
	}
} // }}}

//调度执行信息结构
type ExecSchedule struct { // {{{
	lock           sync.Mutex
	batchId        string              //批次ID，规则scheduleId + 周期开始时间(不含周期内启动时间)
	schedule       *Schedule           //调度
	startTime      *time.Time          //开始时间
	endTime        *time.Time          //结束时间
	state          int8                //状态 0.不满足条件未执行 1. 执行中 2. 暂停 3. 完成 4.意外中止
	result         float32             //结果,调度中执行成功任务的百分比
	execType       int8                //执行类型 1. 自动定时调度 2.手动人工调度 3.修复执行
	execJobs       []*ExecJob          //作业执行信息
	execTasks      map[int64]*ExecTask //任务执行信息
	execTaskChan   chan *ExecTask      //taskChan用来传递完成的任务。当一个作业完成后会将自己放入taskChan变量中
	jobCnt         int                 //调度中作业数量
	taskCnt        int                 //调度中任务数量
	successTaskCnt int                 //执行成功任务数量
	failTaskCnt    int                 //执行失败任务数量
	LogId          int                 //调度日志Id
} // }}}

//初始化调度的执行结构，使之包含完整的执行链。
func (es *ExecSchedule) InitExecSchedule() (err error) { // {{{
	defer func() {
		if err := recover(); err != nil {
			var buf bytes.Buffer
			buf.Write(debug.Stack())
			g.L.Errorln("ExecSchedule Run Panic=", buf.String())
			return
		}
	}()
	if err = es.Log(); err != nil {
		return errors.New(fmt.Sprintf("\n[es.InitExecSchedule] Save Log Failed %s", err.Error()))
	}

	for _, j := range es.schedule.Jobs {
		execJob := ExecJobWarper(es.batchId, j)
		err = execJob.InitExecJob(es)
		if err != nil {
			return errors.New(fmt.Sprintf("\n[es.InitExecSchedule] %s", err.Error()))
		}
		es.execJobs = append(es.execJobs, execJob)
		es.taskCnt = es.taskCnt + execJob.taskCnt
	}
	return err
} // }}}

func (es *ExecSchedule) AddExecTask(tasks []*Task) (err error) {

	var i int
	for k, ej := range es.execJobs {
		if ej.job.Id == tasks[0].JobId {
			i = k
		}
	}
	etasks := make([]*ExecTask, 0, len(tasks))
	for _, t := range tasks {
		ej := es.execJobs[i]
		et := ExecTaskWarper(ej, t)
		ej.execTasks[t.Id] = et
		es.execTasks[t.Id] = et
		es.taskCnt = es.taskCnt + 1
		ej.taskCnt = ej.taskCnt + 1
		etasks = append(etasks, et)
	}

	for _, et := range etasks {
		if err = et.InitExecTask(es); err != nil {
			e := fmt.Sprintf("\n[ej.InitExecTask] %s %s", et.task.Name, err.Error())
			return errors.New(e)
		}
	}
	return nil
}

//ExecSchedule执行前状态记录
func (es *ExecSchedule) Start() (err error) { // {{{
	es.startTime = NowTimePtr()
	es.state = 1
	if err = es.Log(); err != nil {
		es.state = 4
		err = errors.New(fmt.Sprintf("\n[es.Start] %s", err.Error()))
	}
	g.L.Infoln(es.schedule.Name, "is start batchId=[", es.batchId, "]")

	return err
} // }}}

//调度中的一个任务完成，更新状态。
//当调度中全部任务完成后，将调度执行体从全局列表中移除，并设置下次启动时间。
func (es *ExecSchedule) TaskDone(et *ExecTask) (finish bool, err error) { // {{{

	//计算任务完成百分比
	s := es.schedule
	es.result = float32(s.TaskCnt-es.taskCnt) / float32(s.TaskCnt)
	if es.taskCnt == 0 { //调度结束
		//g.Schedules.RemoveExecSchedule(es.batchId)
		s.delExecSchChan <- es.batchId
		//全部完成后，写入日志存储至数据库，设置下次启动时间
		es.endTime = NowTimePtr()
		es.state = 3
		if err = es.Log(); err != nil {
			es.state = 4
			return true, errors.New(fmt.Sprintf("\n[es.TaskDone] %s", err.Error()))
		}

		g.L.Infoln("schedule ", s.Name, " is end ", " batchId=", es.batchId,
			" success=", es.successTaskCnt, " fail=", es.failTaskCnt, " result=", es.result)

		return true, nil
	}

	return false, err
} // }}}

//ExecSchedule.Run()方法执行调度任务。
//过程中会维护一个Chan *ExecTask类型变量staskChan，用来传递执行完成的Task。
//通过遍历Schedule下的全部Task，找出可执行的Task(依赖列表为空的Task)，启动线程执行task.Run
//方法，并将staskChan传给它。当Task执行结束后会把自己放入staskChan中，处理的另一部分监控着
//staskChan，从其中取出执行完毕的task后，会从其它任务的依赖列表中将已执行完毕的task删除，
//并重新找出依赖列表为空的task，启动线程运行它的Run方法。
//全部执行结束后，设置Schedule的下次启动时间。
func (es *ExecSchedule) Run() { // {{{
	var err error
	defer func() {
		if err := recover(); err != nil {
			var buf bytes.Buffer
			buf.Write(debug.Stack())
			g.L.Errorln("ExecSchedule Run Panic=", buf.String())
			return
		}
	}()
	if err = es.Start(); err != nil {
		g.L.Warningln(fmt.Sprintf("\n[es.Run] %s", err.Error()))
		return
	}
	if err = es.RunTasks(); err != nil {
		g.L.Warningln(fmt.Sprintf("\n[es.Run] %s", err.Error()))
		return
	}
	//不断轮询taskChan中的信息，直到最后一个任务完成
	//调用执行结构的Timer方法，并退出线程。
	for {
		select {
		case et := <-es.execTaskChan:
			es.taskCnt--

			//将该任务从其它任务的依赖列表中删除。
			for _, et1 := range es.execTasks {

				//任务执行失败，将依赖的下级任务状态设置为2（暂停）
				if et.state != 3 && et.state != 5 {
					if _, ok := et1.relExecTasks[et.task.Id]; ok && et1.state != 2 {
						et1.state = 2
					}
				}

				delete(et1.relExecTasks, et.task.Id)
				delete(et1.nextExecTasks, et.task.Id)
			}

			if et.state == 3 || et.state == 5 { //任务执行成功或可以忽略
				es.successTaskCnt++
			} else if et.state == 2 {
				es.failTaskCnt++ //暂停的也计入失败数量
				g.L.Debugln("task", et.task.Name, "is pause batchTaskId[", et.batchTaskId, "] state=", et.state)
			} else {
				es.failTaskCnt++
				g.L.Debugln("task", et.task.Name, "is fail batchTaskId[", et.batchTaskId, "] state=", et.state)
			}

			if err = et.execJob.TaskDone(et); err != nil {
				g.L.Warningln(fmt.Sprintf("\n[es.Run] %s", err.Error()))
				return
			}

			finish := false
			if finish, err = es.TaskDone(et); finish && err == nil {
				return
			} else if err != nil {
				g.L.Warningln(fmt.Sprintf("\n[es.Run] %s", err.Error()))
				return
			}

			if err = es.RunTasks(); err != nil {
				g.L.Warningln(fmt.Sprintf("\n[es.Run] %s", err.Error()))
				return
			}

		}
	}

} // }}}

//执行参数ets中符合运行条件的任务
func (es *ExecSchedule) RunTasks() (err error) { // {{{
	//启动独立的任务
	for _, et := range es.execTasks {
		//依赖任务列表为空，任务可以执行
		if len(et.relExecTasks) == 0 && (et.state == 0 || et.state == 2) {

			//任务所属作业开始时间为空，设置作业启动信息
			if err = et.execJob.Start(); err != nil {
				es.state = 4
				return errors.New(fmt.Sprintf("\n[es.RunTasks] %s", err.Error()))
			}

			//将该任务从任务列表中删除。
			delete(es.execTasks, et.task.Id)

			//执行任务，完成后任务会放入taskChan中
			go et.Run(es.execTaskChan)
		}
	}

	return err
} // }}}

//Pause暂停调度执行
func (es *ExecSchedule) Pause() { // {{{
	es.lock.Lock()
	defer es.lock.Unlock()
	for _, t := range es.execTasks {
		t.state = 2
	}

} // }}}

//作业执行信息结构
type ExecJob struct { // {{{
	batchJobId string     //作业批次ID，批次ID + 作业ID
	batchId    string     //批次ID，规则scheduleId + 周期开始时间(不含周期内启动时间)
	job        *Job       //作业
	startTime  *time.Time //开始时间
	endTime    *time.Time //结束时间
	state      int8       //状态 0.不满足条件未执行 1. 执行中 2. 暂停 3. 完成 4.意外中止
	result     float32    //结果执行成功任务的百分比
	//nextJob    *ExecJob            //下一个作业
	execType  int8                //执行类型1. 自动定时调度 2.手动人工调度 3.修复执行
	execTasks map[int64]*ExecTask //任务执行信息
	taskCnt   int                 //作业中任务数量
	LogId     int                 //调度日志Id
} // }}}

//根据传入的batchId和Job参数来构建一个调度的执行结构，并返回。
func ExecJobWarper(batchId string, j *Job) *ExecJob { // {{{
	return &ExecJob{
		batchJobId: fmt.Sprintf("%s.%d", batchId, j.Id),
		batchId:    batchId,
		job:        j,
		state:      0,
		result:     0,
		execType:   j.ExecType,
		execTasks:  make(map[int64]*ExecTask, 0),
	}
} // }}}

//初始化作业执行链，并返回。
func (ej *ExecJob) InitExecJob(es *ExecSchedule) (err error) { // {{{
	if err = ej.Log(); err != nil {
		e := fmt.Sprintf("\n[ej.InitExecJob] %s %s", ej.job.Name, err.Error())
		return errors.New(e)
	}

	//构建当前作业中的任务执行结构
	for _, t := range ej.job.Tasks { // {{{
		et := ExecTaskWarper(ej, t)
		if es.schedule.NextStart == t.NextRunTime {
			ej.execTasks[t.Id] = et
			es.execTasks[t.Id] = et
		}

	} // }}}
	ej.taskCnt = len(ej.execTasks)
	for _, et := range es.execTasks {
		if err = et.InitExecTask(es); err != nil {
			e := fmt.Sprintf("\n[ej.InitExecJob] %s %s", ej.job.Name, err.Error())
			return errors.New(e)
		}
	}
	return nil

} // }}}

//设置ExecJob的状态为开始，并记录到log中
func (ej *ExecJob) Start() (err error) { // {{{
	if ej.startTime == nil {
		ej.startTime = NowTimePtr()
		ej.state = 1
		if err = ej.Log(); err != nil {
			ej.state = 4
			err = errors.New(fmt.Sprintf("\n[ej.Start] %s", err.Error()))
		}
		g.L.Infoln("job ", ej.job.Name, " is start ", " batchJobId[", ej.batchJobId, "]")
	}

	return err
} // }}}

func (ej *ExecJob) TaskDone(et *ExecTask) (err error) { // {{{
	delete(ej.execTasks, et.task.Id)
	ej.taskCnt--
	//计算任务完成百分比
	ej.result = float32(ej.job.TaskCnt-ej.taskCnt) / float32(ej.job.TaskCnt)
	if ej.taskCnt == 0 { //作业结束
		ej.endTime = NowTimePtr()
		ej.state = 3
		if err = ej.Log(); err != nil {
			ej.state = 4
			err = errors.New(fmt.Sprintf("\n[ej.TaskDone] %s", err.Error()))
		}
		g.L.Debugln("job ", ej.job.Name, " is end ", " batchJobId[", ej.batchJobId, "] result=", ej.result)
	}

	return err
} // }}}

//任务执行信息结构
type ExecTask struct { // {{{
	batchTaskId   string     //任务批次ID，作业批次ID + 任务ID
	batchJobId    string     //作业批次ID，批次ID + 作业ID
	batchId       string     //批次ID，规则scheduleId + 周期开始时间(不含周期内启动时间)
	task          *Task      //任务
	startTime     *time.Time //开始时间
	endTime       *time.Time //结束时间
	state         int8       //状态 0.初始状态 1. 执行中 2. 暂停 3. 完成 4.意外中止 5.忽略
	execType      int8       //执行类型 1. 自动定时调度 2.手动人工调度 3.修复执行
	execJob       *ExecJob   //任务所属作业
	output        string     //任务输出
	stderr        string
	errstr        string              //任务输出
	nextExecTasks map[int64]*ExecTask //下级任务执行信息
	relExecTasks  map[int64]*ExecTask //依赖的任务
	LogId         int                 //调度日志Id
	Retry         int
} // }}}

//根据传入的batchId和Job参数来构建一个调度的执行结构，并返回。
func ExecTaskWarper(ej *ExecJob, t *Task) *ExecTask { // {{{
	return &ExecTask{
		batchTaskId:   fmt.Sprintf("%s.%d", ej.batchJobId, t.Id),
		batchJobId:    ej.batchJobId,
		batchId:       ej.batchId,
		task:          t,
		state:         0,
		execType:      t.TaskType,
		Retry:         t.Retry,
		execJob:       ej,
		relExecTasks:  make(map[int64]*ExecTask),
		nextExecTasks: make(map[int64]*ExecTask),
	}
} // }}}

//初始化Task执行结构
func (et *ExecTask) InitExecTask(es *ExecSchedule) error { // {{{

	if err := et.Log(); err != nil {
		e := fmt.Sprintf("\n[et.InitExecTask] %s %s", et.task.Name, err.Error())
		return errors.New(e)
	}
	for _, relTask := range et.task.RelTasks {
		retask := es.execTasks[relTask.Id]
		et.relExecTasks[relTask.Id] = retask
		fmt.Println("rel :", relTask.Id, retask)
		//将execTask设置为依赖任务的下级任务
		retask.nextExecTasks[et.task.Id] = et
	}
	return nil
} // }}}

type Reply struct { // {{{
	Err    string //错误信息
	Stdout string //标准输出
	Stderr string //标准输出
} // }}}

//Run方法负责执行任务。
//首先会判断是否符合执行条件，符合则执行
//执行时会从任务执行结构中取出需要执行的信息，通过RPC发送给执行模块执行。
//完成后更新执行信息，并将任务置入taskChan变量中，供后续处理。
func (et *ExecTask) Run(taskChan chan *ExecTask) { // {{{
	rl := &Reply{}
	defer func() { // {{{
		if err := recover(); err != nil {
			var buf bytes.Buffer
			buf.Write(debug.Stack())
			et.endTime = NowTimePtr()
			et.state = 4
			g.L.Warningln("task run error", "batchTaskId[", et.batchTaskId, "] TaskName=",
				et.task.Name, "output=", et.output, "err=", err, " stack=", buf.String())
			et.Log()

			taskChan <- et
			return
		}
	}() // }}}

	//暂停状态的处理
	if et.state == 2 {
		et.Log()
		taskChan <- et
		return
	}

	et.startTime = NowTimePtr()
	et.state = 1
	et.Log()
	g.L.Debugln("task", et.task.Name,
		"is start batchTaskId[", et.batchTaskId, "] cmd =",
		et.task.Cmd)

	//执行任务
	task := et.task
	et.state = 3
	var client *rpc.Client
	var err error
	for i := et.Retry; i > 0; i -= 1 {
		client, err = rpc.Dial("tcp", et.task.Address+g.Port)
		if err == nil {
			err = client.Call("CmdExecuter.Run", task, &rl)
			if err == nil {
				if rl.Err != "" {
					//et.output = rl.Err
					et.errstr = rl.Err
					g.L.Infoln("task", et.task.Name, "is error", rl.Err == "")
				} else {
					client.Close()
					break
				}
			} else {
				g.L.Errorf("task", et.task.Name, "is error", err.Error())
			}
			client.Close()
		} else {
			g.L.Errorf("connect task.Address[%s] error %s\n", et.task.Address+g.Port,
				err.Error())
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil || rl.Err != "" {
		et.state = 4
	}

	et.output = rl.Stdout
	et.stderr = rl.Stderr
	et.endTime = NowTimePtr()
	et.Log()

	g.L.Debugln("task", et.task.Name, "is end batchTaskId[", et.batchTaskId, "] state =",
		et.state, "StartTime", et.startTime, "EndTime", et.endTime)

	taskChan <- et

} // }}}

//ExecSchedule.Restore(batchId string)方法修复执行指定的调度。
//根据传入的batchId，构建调度执行结构，并调用Run方法执行其中的任务
func Restore(batchId string, scdId int64) (err error) { // {{{

	g.L.Infoln("Restore schedule by ", " batchid[", batchId, "] scdId=", scdId)

	//获取执行成功的Task
	successTaskId := getSuccessTaskId(batchId)

	//创建ExecSchedule结构
	s := g.Schedules.ScheduleList[scdId]
	execSchedule := &ExecSchedule{
		batchId:   batchId,
		schedule:  s,
		state:     1,
		result:    0,
		execType:  3,
		jobCnt:    s.JobCnt,
		taskCnt:   s.TaskCnt,
		execTasks: make(map[int64]*ExecTask), //设置任务列表
	}
	err = execSchedule.InitExecSchedule()

	//删除成功的任务
	for _, tId := range successTaskId {
		t := execSchedule.execTasks[tId]
		for _, nextaks := range t.nextExecTasks {
			delete(nextaks.relExecTasks, t.task.Id)
		}
		delete(execSchedule.execTasks, tId)
		execSchedule.taskCnt--
	}

	//设置作业、任务的初始状态
	for _, t := range execSchedule.execTasks {
		t.execType = 3
		t.state = 1
		t.execJob.execType = 3
		t.execJob.state = 1
	}
	g.L.Infoln("schedule will restore")

	//执行
	execSchedule.Run()
	g.L.Infoln("schedule was restored")

	return nil
} // }}}
