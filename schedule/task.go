package schedule

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// 任务信息结构
type Task struct { // {{{
	Id           int64  // 任务的ID
	Address      string // 任务的执行地址
	Name         string // 任务名称
	TaskType     int8   // 任务类型
	ScheduleCyc  string `json:"-"` //调度周期
	TaskCyc      string //调度周期
	Cronstr      string //`json:"-"`
	Retry        int
	Concurrent   int
	Disabled     int8              //`json:"-"`
	Priority     int16             //`json:"-"`
	StartSecond  time.Duration     //周期内启动时间
	Cmd          string            // 任务执行的命令或脚本、函数名等。
	Desc         string            //任务说明
	TimeOut      int64             // 设定超时时间，0表示不做超时限制。单位秒
	Attr         map[string]string `json:"-"` // 任务的属性信息
	JobId        int64             `json:"-"` //所属作业ID
	RelTasksId   []int64           //依赖的任务Id
	RelTasks     map[string]*Task  //`json:"-"` //依赖的任务
	RelTaskCnt   int64             //依赖的任务数量
	CreateUserId int64             //创建人
	CreateTime   *time.Time        //创人
	ModifyUserId int64             //修改人
	ModifyTime   *time.Time        //修改时间
	NextRunTime  time.Time
	PreRunTime   time.Time
	timer        Timer
} // }}}

//根据Task.Id从元数据库获取信息初始化Task结构，包含以下动作
//初始化Task基本信息
//      Task属性信息
//      Task的参数信息
//      依赖的Task列表
//失败返回错误信息。
func (t *Task) InitTask(s *Schedule) error { // {{{
	g.L.Debugf("InitTask[%s] Start ...\n", t.Name)
	t.GetTask()
	t.getRelTaskId()
	for _, rtid := range t.RelTasksId {
		rt := s.GetTaskById(rtid)
		idkey := strconv.FormatInt(rtid, 10)
		t.RelTasks[idkey] = rt
		if rt == nil {
			e := fmt.Sprintf("[t.InitTask] Task [%d] not found RelTask [%s] .\n", t.Id, idkey)
			g.L.Warningln(e)
			continue
		}
		t.RelTaskCnt++
	}
	s.addTaskList(t)
	tm, err := Parse(t.Cronstr)
	if err != nil {
		return err
	}
	t.timer = tm
	g.L.Debugf("InitTask[%s] End ...\n", t.Name)
	return nil
} // }}}

func (t *Task) GetTask() error { // {{{

	err := t.getTask()
	if err != nil {
		e := fmt.Sprintf("\n[t.InitTask] %s.", err.Error())
		return errors.New(e)
	}

	err = t.getTaskAttr()
	if err != nil {
		e := fmt.Sprintf("\n[t.InitTask] %s.", err.Error())
		return errors.New(e)
	}

	t.RelTasksId = make([]int64, 0)
	t.RelTasks = make(map[string]*Task)
	t.RelTaskCnt = 0

	return nil
} // }}}

func (t *Task) NextTime(now time.Time) time.Time {
	if t.Disabled != 0 || t.TaskType == 0 {
		t.NextRunTime = time.Time{}
		return t.NextRunTime
	}

	if t.TaskType == 1 {
		if t.Cronstr != "" {
			t.NextRunTime = t.timer.Next(now)
		} else {
			t.NextRunTime, _ = getCountDownTime(t.TaskCyc, []int{0}, []time.Duration{t.StartSecond})
		}
	} else if t.TaskType == 2 && len(t.RelTasks) > 0 {
		idkey := strconv.FormatInt(t.RelTasksId[0], 10)
		t.NextRunTime = t.RelTasks[idkey].NextTime(now)
	} else {
		t.NextRunTime = time.Time{}
	}
	return t.NextRunTime
}

//更新Task信息到元数据库。
//更新基本信息后，更新参数信息
func (t *Task) UpdateTask() error { // {{{
	err := t.update()
	if err != nil {
		e := fmt.Sprintf("\n[t.UpdateTask] %s.", err.Error())
		return errors.New(e)
	}

	return err
} // }}}

func (t *Task) Refresh(s *Schedule) error { // {{{
	g.L.Debugf("Refresh[%s] Start ...\n", t.Name)
	i := -1
	for k, task := range s.Tasks {
		if task.Id == t.Id {
			i = k
		}
	}

	err := t.getTask()
	if err != nil {
		t := s.Tasks[i]
		s.Tasks = append(s.Tasks[0:i], s.Tasks[i+1:]...)
		s.TaskCnt = len(s.Tasks)
		j, er := s.GetJobById(t.JobId)
		if er != nil {
			e := fmt.Sprintf("\n[s.Refresh] get job [%d] error %s", t.JobId, er.Error())
			return errors.New(e)
		}

		err := j.DeleteTask(t.Id)
		if err != nil {
			e := fmt.Sprintf("\n[s.Refresh] Refresh error %s", err.Error())
			return errors.New(e)
		}
	}

	err = t.getTaskAttr()
	if err != nil {
		e := fmt.Sprintf("\n[t.Refresh] %s.", err.Error())
		return errors.New(e)
	}

	t.RelTasksId = make([]int64, 0)
	t.RelTasks = make(map[string]*Task)
	t.RelTaskCnt = 0

	err = t.getRelTaskId()
	for _, rtid := range t.RelTasksId {
		rt := s.GetTaskById(rtid)
		idkey := strconv.FormatInt(rtid, 10)
		t.RelTasks[idkey] = rt
		if rt == nil {
			e := fmt.Sprintf("[t.Refresh] Task [%d] not found RelTask [%s] .\n", t.Id, idkey)
			g.L.Warningln(e)
			continue
		}
		t.RelTaskCnt++

	}
	tm, err := Parse(t.Cronstr)
	if err != nil {
		return err
	}
	t.timer = tm

	if i == -1 {
		s.addTaskList(t)
		s.TaskCnt = len(s.Tasks)
		j, err := s.GetJobById(t.JobId)
		if err != nil {
			e := fmt.Sprintf("\n[s.AddTask] not found job by id %d", t.JobId)
			return errors.New(e)
		}
		idkey := strconv.FormatInt(t.Id, 10)
		j.Tasks[idkey] = t
		j.TaskCnt++
		t.NextTime(time.Now())
	}
	g.L.Debugf("Refresh[%s] End ...\n", t.Name)
	return nil
}

//AddTask方法持久化当前的Task信息。
//调用add方法将Task基本信息持久化。
//完成后处理作业关联信息、Task依赖关系、参数列表。
func (t *Task) AddTask() (err error) { // {{{
	err = t.add()
	if err != nil {
		e := fmt.Sprintf("\n[t.AddTask] %s.", err.Error())
		return errors.New(e)
	}

	for _, rt := range t.RelTasksId {
		err = t.addRelTask(rt)
		if err != nil {
			e := fmt.Sprintf("\n[t.AddTask] %s.", err.Error())
			return errors.New(e)
		}
	}

	return err
} // }}}

//删除依赖的任务关系
func (t *Task) DeleteRelTask(relid int64) error { // {{{

	err := t.deleteRelTask(relid)
	if err != nil {
		e := fmt.Sprintf("\n[t.DeleteRelTask] %s.", err.Error())
		return errors.New(e)
	}

	return err
} // }}}

//增加依赖的任务
func (t *Task) AddRelTask(rt *Task) (err error) { // {{{

	err = t.addRelTask(rt.Id)
	if err != nil {
		e := fmt.Sprintf("\n[t.AddRelTask] error %s.", err.Error())
		return errors.New(e)
	}
	return err
} // }}}

//删除Task,依次删除Param、RelTask关系、Task
func (t *Task) Delete() (err error) { // {{{

	for _, rid := range t.RelTasksId {
		err = t.DeleteRelTask(rid)
		if err != nil {
			e := fmt.Sprintf("\n[t.Delete] %s.", err.Error())
			return errors.New(e)
		}
	}

	err = t.deleteTask()
	if err != nil {
		e := fmt.Sprintf("\n[t.Delete] error %s.", err.Error())
		return errors.New(e)
	}
	return err

} // }}}
