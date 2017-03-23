//调度模块，负责从元数据库读取并解析调度信息。
//将需要执行的任务发送给执行模块，并读取返回信息。
package schedule

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"sort"
	//"sync"
	"time"
)

var (
	g *GlobalConfigStruct
)

//GlobalConfigStruct结构中定义了程序中的一些配置信息
type GlobalConfigStruct struct { // {{{
	L           *logrus.Logger   //log对象
	HiveConn    *sql.DB          //元数据库链接
	LogConn     *sql.DB          //日志数据库链接
	ManagerPort string           //管理模块的web服务端口
	Port        string           //Schedule与Worker模块通信端口
	Schedules   *ScheduleManager //包含全部Schedule列表的结构
} // }}}

type Timer interface {
	// Return the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
}

//返回GlobalConfigStruct的默认值。
func DefaultGlobal() *GlobalConfigStruct { // {{{
	sc := &GlobalConfigStruct{}
	sc.L = logrus.New()
	sc.L.Formatter = new(logrus.TextFormatter) // default
	sc.L.Level = logrus.DebugLevel
	sc.Port = ":3128"
	sc.ManagerPort = ":3000"
	sc.Schedules = &ScheduleManager{Global: sc, ExecScheduleList: make(map[string]*ExecSchedule)}
	return sc
} // }}}

//ScheduleManager通过成员ScheduleList持有全部的Schedule。
//并提供获取、增加、删除以及启动、停止Schedule的功能。
type ScheduleManager struct { // {{{
	ScheduleList     []*Schedule //全部的调度列表
	DefaultScd       *Schedule
	ExecScheduleList map[string]*ExecSchedule //当前执行的调度列表
	Global           *GlobalConfigStruct      //配置信息
} // }}}

//初始化ScheduleList，设置全局变量g
func (sl *ScheduleManager) InitScheduleList() { // {{{
	g = sl.Global
	//从元数据库读取调度信息,初始化调度列表
	err := sl.getAllSchedules()
	if err != nil {
		e := fmt.Sprintf("[sl.InitScheduleList] init scheduleList error %s.\n", err.Error())
		g.L.Fatalln(e)
	}
	def_scd := &Schedule{
		Name: "DefaultScd",
		Cyc:  "mi",
	}
	sl.ScheduleList = append(sl.ScheduleList, def_scd)

} // }}}

//增加一个调度执行结构
func (sl *ScheduleManager) AddExecSchedule(es *ExecSchedule) { // {{{
	sl.ExecScheduleList[es.batchId] = es
	return
} // }}}

//移除一个调度执行结构
func (sl *ScheduleManager) RemoveExecSchedule(batchId string) { // {{{
	/*var lock sync.Mutex
	lock.Lock()
	defer lock.Unlock()*/

	delete(sl.ExecScheduleList, batchId)
} // }}}

//开始监听Schedule，遍历列表中的Schedule并启动它的Timer方法。
func (sl *ScheduleManager) StartListener() { // {{{
	for _, scd := range sl.ScheduleList {
		//从元数据库初始化调度链信息
		err := scd.InitSchedule()
		if err != nil {
			g.L.Warningf("[sl.StartListener] init schedule [%d] error %s.\n", scd.Id, err.Error())
			return
		}
		//启动监听，按时启动Schedule
		go scd.Timer()
	}

}

//启动指定的Schedule，从ScheduleList中获取到指定id的Schedule后，从元数据库获取
//Schedule的信息初始化一下调度链，然后调用它自身的Timer方法，启动监听。
//失败返回error信息。
func (sl *ScheduleManager) StartScheduleById(id int64) error { // {{{
	s := sl.GetScheduleById(id)
	if s == nil {
		e := fmt.Sprintf("\n[sl.StartScheduleById] start schedule. not found schedule by id %d", id)
		return errors.New(e)
	}

	//从元数据库初始化调度链信息
	err := s.InitSchedule()
	if err != nil {
		e := fmt.Sprintf("\n[sl.StartScheduleById] init schedule [%d] error %s.", id, err.Error())
		return errors.New(e)
	}

	//启动监听，按时启动Schedule
	go s.Timer()

	return nil
} // }}}

//查找当前ScheduleList列表中指定id的Schedule，并返回。
//查不到返回nil
func (sl *ScheduleManager) GetScheduleById(id int64) *Schedule { // {{{
	for _, s := range sl.ScheduleList {
		if s.Id == id {
			return s
		}
	}
	return nil
} // }}}

//增加Schedule，将参数中的Schedule加入的列表中，并调用其Add方法持久化。
func (sl *ScheduleManager) AddSchedule(s *Schedule) error { // {{{
	err := s.Add()
	if err != nil {
		e := fmt.Sprintf("\n[sl.AddSchedule] %s.", err.Error())
		return errors.New(e)
	}
	sl.ScheduleList = append(sl.ScheduleList, s)

	return nil
} // }}}

//从当前ScheduleList列表中移除指定id的Schedule。
//完成后，调用Schedule自身的Delete方法，删除其中的Job、Task信息并做持久化操作。
//失败返回error信息
func (sl *ScheduleManager) DeleteSchedule(id int64) error { // {{{
	i := -1
	for k, ss := range sl.ScheduleList {
		if ss.Id == id {
			i = k
		}
	}

	if i == -1 {
		e := fmt.Sprintf("\n[sl.DeleteSchedule] delete error. not found schedule by id %d", id)
		return errors.New(e)
	}

	s := sl.ScheduleList[i]
	sl.ScheduleList = append(sl.ScheduleList[0:i], sl.ScheduleList[i+1:]...)

	err := s.Delete()
	if err != nil {
		e := fmt.Sprintf("\n[sl.DeleteSchedule] delete schedule [%d %s] error. %s", id, s.Name, err.Error())
		return errors.New(e)
	}

	return nil
} // }}}

//调度信息结构
type Schedule struct { // {{{
	Id             int64     `json:"-"` //调度ID
	Name           string    `json:"-"` //调度名称
	Count          int8      `json:"-"` //调度次数
	Cyc            string    `json:"-"` //调度周期
	NextStart      time.Time `json:"-"` //下次启动时间
	TimeOut        int64     `json:"-"` //最大执行时间
	Jobs           []*Job    `json:"-"` //作业列表
	Tasks          []*Task   //任务列表
	isRefresh      chan bool `json:"-"` //是否刷新标志
	Desc           string    `json:"-"` //调度说明
	JobCnt         int       `json:"-"` //调度中作业数量
	TaskCnt        int       //调度中任务数量
	CreateUserId   int64     `json:"-"` //创建人
	CreateTime     time.Time `json:"-"` //创人
	ModifyUserId   int64     `json:"-"` //修改人
	ModifyTime     time.Time `json:"-"` //修改时间
	updateTaskChan chan *Task
	doTaskChan     chan *Task
	delExecSchChan chan string
} // }}}

//按时启动Schedule，Timer中会根据Schedule的周期以及启动时间计算下次
//启动的时间，并依据此设置一个定时器按时唤醒，Schedule唤醒后，会重新
//从元数据库初始化一下信息，生成执行结构ExecSchedule，执行其Run方法
func (s *Schedule) Timer() { // {{{
	if s.Cyc == "" {
		g.L.Warnf("[s.Timer] Schedule [%s] Cyc is not set!", s.Name)
		return
	}
	now := time.Now()
	for _, t := range s.Tasks {
		t.NextTime(now)
	}
	var countDown time.Duration
	for {

		sort.Sort(byTime(s.Tasks))
		countDown = time.Duration(0)
		if len(s.Tasks) == 0 || s.Tasks[0].NextRunTime.IsZero() {
			countDown, _ = getCountDown(s.Cyc, []int{0}, []time.Duration{time.Duration(0)})
			s.NextStart = time.Now().Add(countDown)
		} else {
			for _, t := range s.Tasks {
				if !t.NextRunTime.IsZero() {
					s.NextStart = s.Tasks[0].NextRunTime
					countDown = s.NextStart.Sub(now)
					break
				}
			}
		}

		select {
		case <-time.After(countDown):
			es := ExecScheduleWarper(s)
			g.Schedules.AddExecSchedule(es)
			//构建执行结构链
			err := es.InitExecSchedule()
			if err != nil {
				g.L.Warnf("[s.Timer] Init Execschedule [%d %s] error %s.\n", s.Id, s.Name, err.Error())
			} else {
				//从元数据库初始化调度链信息
				g.L.Infof("[s.Timer] schedule [%d %s] is start.\n", s.Id, s.Name)
				//启动线程执行调度任务
				go es.Run()
			}
			now = time.Now()
			for _, t := range s.Tasks {
				if t.NextRunTime == s.NextStart {
					t.NextTime(now)
				}
			}
		case t := <-s.doTaskChan:
			t.NextRunTime = now
		case t := <-s.updateTaskChan:
			t.Refresh(s)
		case batchID := <-s.delExecSchChan:
			g.Schedules.RemoveExecSchedule(batchID)
		case <-s.isRefresh:
			g.L.Infof("[s.Timer] schedule [%d %s] is refresh.\n", s.Id, s.Name)
			return
		}
	}
	return
} // }}}

//从元数据库初始化Schedule结构，先从元数据库获取Schedule的信息，完成后
//根据其中的Jobid继续从元数据库读取job信息，并初始化。完成后继续初始化下级Job，
//同时将初始化完成的Job和Task添加到Schedule的Jobs、Tasks成员中。
func (s *Schedule) InitSchedule() error { // {{{
	g.L.Infof("Init Schedule[%s] Start ...\n", s.Name)
	err := s.getSchedule()
	s.updateTaskChan = make(chan *Task, 2)
	s.doTaskChan = make(chan *Task, 2)
	//s.delTaskChan = make(chan int64, 2)
	if err != nil {
		e := fmt.Sprintf("\n[s.InitSchedule] get schedule [%d] error %s.", s.Id, err.Error())
		return errors.New(e)
	}
	s.getJobs()
	for _, tj := range s.Jobs {
		tj.ScheduleId, tj.ScheduleCyc = s.Id, s.Cyc
		if err = tj.InitJob(s); err != nil {
			e := fmt.Sprintf("\n[s.InitSchedule] init job [%d] error %s.", tj.Id, err.Error())
			return errors.New(e)
		}
	}
	g.L.Infof("Init Schedule[%s] End ...\n", s.Name)
	return nil
} // }}}
func (s *Schedule) UpdateTask(t *Task) {
	s.updateTaskChan <- t
}

func (s *Schedule) DoTask(tasks *Task) {
	s.doTaskChan <- tasks
}

//刷新Schedule
func (s *Schedule) refresh() { // {{{
	//发送消息停止监听
	s.isRefresh <- true
	//启动监听，按时启动Schedule
	go s.Timer()

	return
} // }}}

//addTaskList将传入的*Task添加到*Schedule.Tasks中
func (s *Schedule) addTaskList(t *Task) { // {{{
	s.Tasks = append(s.Tasks, t)
	s.TaskCnt++
} // }}}

//GetTaskById根据传入的id查找Tasks中对应的Task，没有则返回nil。
func (s *Schedule) GetTaskById(id int64) *Task { // {{{
	for _, v := range s.Tasks {
		if v.Id == id {
			return v
		}
	}
	return nil
} // }}}

//DeleteTask方法用来删除指定id的Task。首先会根据传入参数在Schedule的Tasks列
//表中查出对应的Task。然后将其从Tasks列表中去除，将其从所属Job中去除，调用
//Task的Delete方法删除Task的依赖关系，完成后删除元数据库的信息。
//没找到对应Task或删除失败，返回error信息。
func (s *Schedule) DeleteTask(id int64) error { // {{{
	i := -1
	for k, task := range s.Tasks {
		if task.Id == id {
			i = k
		}
	}
	if i == -1 {
		e := fmt.Sprintf("\n[s.DeleteTask] not found task by id %d", id)
		return errors.New(e)
	}

	t := s.Tasks[i]
	s.Tasks = append(s.Tasks[0:i], s.Tasks[i+1:]...)
	s.TaskCnt = len(s.Tasks)

	j, er := s.GetJobById(t.JobId)
	if er != nil {
		e := fmt.Sprintf("\n[s.DeleteTask] get job [%d] error %s", id, er.Error())
		return errors.New(e)
	}

	err := j.DeleteTask(t.Id)
	if err != nil {
		e := fmt.Sprintf("\n[s.DeleteTask] DeleteTask error %s", err.Error())
		return errors.New(e)
	}

	err = t.Delete()
	if err != nil {
		e := fmt.Sprintf("\n[s.DeleteTask] schedule [%d] Delete error %s.", err.Error())
		return errors.New(e)
	}

	return err
} // }}}

//GetJobById遍历Jobs列表，返回调度中指定Id的Job，若没找到返回nil
func (s *Schedule) GetJobById(id int64) (*Job, error) { // {{{
	for _, j := range s.Jobs {
		if j.Id == id {
			return j, nil
		}
	}
	e := fmt.Sprintf("\n[s.GetJobById] not found job  [%d] .", id)
	return nil, errors.New(e)
} // }}}

//在调度中添加一个Job，AddJob会接收传入的Job类型的参数，并调用它的
//Add()方法进行持久化操作。成功后把它添加到调度链中，添加时若调度
//下无Job则将Job直接添加到调度中，否则添加到调度中的任务链末端。
func (s *Schedule) AddJob(job *Job) error { // {{{
	err := job.add()
	if err != nil {
		e := fmt.Sprintf("\n[s.AddJob] %s.", err.Error())
		return errors.New(e)
	}
	if len(s.Jobs) > 0 {
		j := s.Jobs[len(s.Jobs)-1]
		j.NextJob, j.NextJobId, job.PreJob = job, job.Id, j
		if err = j.update(); err != nil {
			e := fmt.Sprintf("\n[s.AddJob] update job [%d] error %s.", job.Id, err.Error())
			return errors.New(e)
		}
	}
	s.Jobs = append(s.Jobs, job)
	s.JobCnt = len(s.Jobs)
	return err
} // }}}

//UpdateJob用来在调度中添加一个Job
//UpdateJob会接收传入的Job类型的参数，修改调度中对应的Job信息，完成后
//调用Job自身的update方法进行持久化操作。
func (s *Schedule) UpdateJob(job *Job) error { // {{{
	j, err := s.GetJobById(job.Id)
	if err != nil {
		e := fmt.Sprintf("\n[s.DeleteTask] not found job by id %d", job.Id)
		return errors.New(e)
	}

	j.Name, j.Desc = job.Name, job.Desc
	j.ModifyTime, j.ModifyUserId = NowTimePtr(), job.ModifyUserId
	err = j.update()
	if err != nil {
		e := fmt.Sprintf("\n[s.UpdateJob] update job [%d] error %s.", j.Id, err.Error())
		return errors.New(e)
	}
	return err
} // }}}

//DeleteJob删除调度中最后一个Job，它会接收传入的Job Id，并查看是否
//调度中最后一个Job，是，检查Job下有无Task，无，则执行删除操作，完成
//后，将该Job的前一个Job的nextJob指针置0，更新调度信息。
//出错或不符条件则返回error信息
func (s *Schedule) DeleteJob(id int64) error { // {{{
	j, err := s.GetJobById(id)
	if err != nil {
		e := fmt.Sprintf("\n[s.DeleteJob] not found job by id %d", id)
		return errors.New(e)
	}
	if j.TaskCnt == 0 && j.NextJobId == 0 {

		if j.PreJobId > 0 {
			pj, er := s.GetJobById(j.PreJobId)
			if er != nil {
				e := fmt.Sprintf("\n[s.DeleteJob] get prejob [%d] error %s", j.PreJobId, er.Error())
				return errors.New(e)
			}

			pj.NextJob, pj.NextJobId = nil, 0
			if err = pj.update(); err != nil {
				e := fmt.Sprintf("\n[s.DeleteJob] update job [%d] to schedule [%d] error %s.", j.Id, s.Id, err.Error())
				return errors.New(e)
			}
		}

		if len(s.Jobs) == 1 {
			s.Jobs = s.Jobs[:0]
		} else {
			s.Jobs = s.Jobs[0 : len(s.Jobs)-1]
		}

		s.JobCnt = len(s.Jobs)
		err = j.deleteJob()
		if err != nil {
			e := fmt.Sprintf("\n[s.DeleteJob] delete job [%d] error %s.", j.Id, err.Error())
			return errors.New(e)
		}
	}
	return err
} // }}}

//增加Schedule信息
func (s *Schedule) Add() error { // {{{
	s.CreateTime, s.ModifyTime = time.Now(), time.Now()
	err := s.add()
	if err != nil {
		e := fmt.Sprintf("\n[s.Add] %s.", err.Error())
		return errors.New(e)
	}
	return nil
} // }}}

//UpdateSchedule方法会将传入参数的信息更新到Schedule结构并持久化到数据库中
//在持久化之前会调用addStart方法将启动列表持久化
func (s *Schedule) UpdateSchedule() error { // {{{
	err := s.AddScheduleStart()
	if err != nil {
		e := fmt.Sprintf("\n[s.UpdateSchedule] addstart error %s.", err.Error())
		return errors.New(e)
	}

	err = s.update()
	if err != nil {
		e := fmt.Sprintf("\n[s.UpdateSchedule] update schedule [%d] error %s.", s.Id, err.Error())
		return errors.New(e)
	}

	s.refresh()
	return err
} // }}}

//Delete方法删除Schedule下的Job、Task信息并持久化。
func (s *Schedule) Delete() error { // {{{
	for _, t := range s.Tasks {
		err := s.DeleteTask(t.Id)
		if err != nil {
			e := fmt.Sprintf("\n[s.Delete] DeleteTask [%d] error %s.", t.Id, err.Error())
			return errors.New(e)
		}
	}

	for _, j := range s.Jobs {
		err := s.DeleteJob(j.Id)
		if err != nil {
			e := fmt.Sprintf("\n[s.Delete] DeleteJob [%d] error %s.", j.Id, err.Error())
			return errors.New(e)
		}
	}

	err := s.deleteSchedule()
	if err != nil {
		e := fmt.Sprintf("\n[s.Delete] deleteSchedule [%d] error %s.", s.Id, err.Error())
		return errors.New(e)
	}
	return nil
} // }}}

//addStart将Schedule的启动列表持久化到数据库
//添加前先调用delStart方法将Schedule中的原有启动列表清空
//需要注意的是：内存中的启动列表单位为纳秒，存储前需要转成秒
//若成功则开始添加，失败返回err信息
func (s *Schedule) AddScheduleStart() error { // {{{

	return nil
} // }}}

//启动时间排序
//算法选择排序
func (s *Schedule) sortStart() { // {{{
	/*	var i, j, k int

		for i = 0; i < len(s.StartMonth); i++ {
			k = i

			for j = i + 1; j < len(s.StartMonth); j++ {
				if s.StartMonth[j] < s.StartMonth[k] {
					k = j
				} else if (s.StartMonth[j] == s.StartMonth[k]) && (s.StartSecond[j] < s.StartSecond[k]) {
					k = j
				}
			}

			if k != i {
				s.StartMonth[k], s.StartMonth[i] = s.StartMonth[i], s.StartMonth[k]
				s.StartSecond[k], s.StartSecond[i] = s.StartSecond[i], s.StartSecond[k]
			}

		}*/

} // }}}
