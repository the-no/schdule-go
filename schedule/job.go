package schedule

import (
	"errors"
	"fmt"
	"time"
)

//作业信息结构
type Job struct { // {{{
	Id           int64  //作业ID
	ScheduleId   int64  //调度ID
	ScheduleCyc  string //调度周期
	ReInit       int    //
	ExecType     int8   //
	Disabled     int8
	Name         string           //作业名称
	Desc         string           //作业说明
	PreJobId     int64            //上级作业ID
	PreJob       *Job             `json:"-"` //上级作业
	NextJobId    int64            //下级作业ID
	NextJob      *Job             `json:"-"` //下级作业
	Tasks        map[string]*Task //作业中的任务
	TaskCnt      int              //调度中任务数量
	CreateUserId int64            //创建人
	CreateTime   *time.Time       //创人
	ModifyUserId int64            //修改人
	ModifyTime   *time.Time       //修改时间
	DeletedAt    *time.Time
} // }}}

//根据Job.Id初始化Job结构，从元数据库获取Job的基本信息初始化后
//继续初始化Job所属的Task列表，同时递归调用自身，初始化下级Job结构
//失败返回error信息。
func (j *Job) InitJob(s *Schedule) error { // {{{
	g.L.Debugf("Init Job[%s] Start ...\n", j.Name)
	err := j.InitTasksForJob(s)
	if err != nil {
		e := fmt.Sprintf("\n[j.InitJob] init task for job [%d] error %s.", j.Id, err.Error())
		return errors.New(e)
	}
	g.L.Debugf("Init Job[%s] End ...\n", j.Name)
	return nil
} // }}}

//初始化Job下的Tasks信息，从元数据库取到Job下所有的TaskId后
//调用方法初始化Task并加至Job的Tasks成员中，同时也添加到全局Tasks列表
//出错返回错误信息
func (j *Job) InitTasksForJob(s *Schedule) error { // {{{
	g.L.Debugf("InitTasksForJob[%s] Start ...\n", j.Name)
	j.Tasks = make(map[string]*Task)

	tasksId, err := j.getTasksId()
	if err != nil {
		e := fmt.Sprintf("\n[j.GetTasks] getTasksId error %s.", err.Error())
		return errors.New(e)
	}

	for _, taskid := range tasksId {
		task := &Task{Id: taskid}
		err := task.InitTask(s)
		if err != nil {
			e := fmt.Sprintf("\n[t.InitTaskForJob] %s.", err.Error())
			return errors.New(e)
		}
		j.Tasks[string(taskid)] = task

		task.ScheduleCyc = j.ScheduleCyc
		j.TaskCnt++
		task.JobId = j.Id
	}
	g.L.Debugf("InitTasksForJob[%s] End ...\n", j.Name)
	return nil
} // }}}

//UpdateTask更新Job中指定Task的信息。
//它会根据参数查找本Job下符合的Task，找到后更新信息
//并调用Task的add方法进行持久化操作。
func (j *Job) UpdateTask(task *Task) (err error) { // {{{
	t, ok := j.Tasks[string(task.Id)]
	if !ok {
		e := fmt.Sprintf("\n[j.UpdateTask] update error. not found task by id %d", task.Id)
		return errors.New(e)
	}
	t.Name, t.Desc, t.Address = task.Name, task.Desc, task.Address
	t.TaskType, t.TaskCyc, t.StartSecond = task.TaskType, task.TaskCyc, task.StartSecond
	t.Cmd, t.TimeOut = task.Cmd, task.TimeOut
	t.Attr, t.ModifyUserId, t.ModifyTime = task.Attr, task.ModifyUserId, NowTimePtr()

	return nil
} // }}}

//删除作业任务映射关系至元数据库
func (j *Job) DeleteTask(taskid int64) (err error) { // {{{
	delete(j.Tasks, string(taskid))
	j.TaskCnt--

	return nil
} // }}}
