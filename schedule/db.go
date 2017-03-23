package schedule

import (
	"errors"
	"fmt"
	"time"
)

//从元数据库获取Schedule列表。
func (sl *ScheduleManager) getAllSchedules() error { // {{{
	sl.ScheduleList = make([]*Schedule, 0)
	//查询全部schedule列表
	sql := `SELECT scd.id,
				scd.scd_name,
				scd.scd_num,
				scd.scd_cyc,
				scd.scd_timeout,
				scd.scd_desc,
				scd.create_user_id,
				scd.create_time,
				scd.modify_user_id,
				scd.modify_time
			FROM scd_schedule scd`
	rows, err := g.HiveConn.Query(sql)
	if err != nil {
		e := fmt.Sprintf("\n[sl.getAllSchedule] run Sql error %s %s", sql, err.Error())
		return errors.New(e)
	}
	g.L.Debugln("[getAllSchedule] ", "\nsql=", sql)

	for rows.Next() {
		scd := &Schedule{
			Jobs:  make([]*Job, 0),
			Tasks: make([]*Task, 0),
		}
		err = rows.Scan(&scd.Id, &scd.Name, &scd.Count, &scd.Cyc, &scd.TimeOut,
			&scd.Desc, &scd.CreateUserId, &scd.CreateTime, &scd.ModifyUserId,
			&scd.ModifyTime)

		sl.ScheduleList = append(sl.ScheduleList, scd)
	}

	return err
} // }}}

//Add方法会将Schedule对象增加到元数据库中。
func (s *Schedule) add() error { // {{{

	sql := `INSERT INTO scd_schedule
            (scd_name, scd_num, scd_cyc,
             scd_timeout,  scd_desc, create_user_id,
             create_time, modify_user_id, modify_time)
		VALUES      ( ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := g.HiveConn.Exec(sql, &s.Name, &s.Count, &s.Cyc,
		&s.TimeOut, &s.Desc, &s.CreateUserId, &s.CreateTime, &s.ModifyUserId, &s.ModifyTime)
	if err != nil {
		e := fmt.Sprintf("[s.add] Query sql [%s] error %s.\n", sql, err.Error())
		return errors.New(e)
	}
	id, _ := result.LastInsertId()
	s.Id = id
	g.L.Debugln("[s.add] schedule", s, "\nsql=", sql)

	return err
} // }}}

//Update方法将Schedule对象更新到元数据库。
func (s *Schedule) update() error { // {{{
	sql := `UPDATE scd_schedule 
		SET  scd_name=?,
             scd_num=?,
             scd_cyc=?,
             scd_timeout=?,
             scd_desc=?,
             create_user_id=?,
             create_time=?,
             modify_user_id=?,
             modify_time=?
		 WHERE id=?`
	_, err := g.HiveConn.Exec(sql, &s.Name, &s.Count, &s.Cyc,
		&s.TimeOut, &s.Desc, &s.CreateUserId, &s.CreateTime, &s.ModifyUserId, &s.ModifyTime, &s.Id)
	if err != nil {
		e := fmt.Sprintf("[s.update] Query sql [%s] error %s.\n", sql, err.Error())
		return errors.New(e)
	}
	g.L.Debugln("[s.update] schedule", s, "\nsql=", sql)

	return err
} // }}}

//Delete方法，删除元数据库中的调度信息
func (s *Schedule) deleteSchedule() error { // {{{
	sql := `Delete FROM scd_schedule WHERE id=?`
	_, err := g.HiveConn.Exec(sql, &s.Id)
	if err != nil {
		e := fmt.Sprintf("[s.deleteSchedule] Query sql [%s] error %s.\n", sql, err.Error())
		return errors.New(e)
	}
	g.L.Debugln("[s.deleteSchedule] schedule", s, "\nsql=", sql)

	return err
} // }}}

//getSchedule，从元数据库获取指定的Schedule信息。
func (s *Schedule) getSchedule() error { // {{{
	if s.Id == 0 {
		s.Name = "DefaultScd"
		s.Cyc = "mi"
		s.Jobs = make([]*Job, 0)
		s.Tasks = make([]*Task, 0)
		s.isRefresh = make(chan bool)
		s.JobCnt, s.TaskCnt = 0, 0
		return nil
	}
	//查询全部schedule列表
	sql := `SELECT scd.id,
				scd.scd_name,
				scd.scd_num,
				scd.scd_cyc,
				scd.scd_timeout,
				scd.scd_desc,
                scd.create_user_id,
                scd.create_time,
                scd.modify_user_id,
                scd.modify_time
			FROM scd_schedule scd
			WHERE scd.id=?`
	rows, err := g.HiveConn.Query(sql, s.Id)
	if err != nil {
		e := fmt.Sprintf("\n[s.getSchedule] run Sql %s error %s", sql, err.Error())
		return errors.New(e)
	}
	g.L.Debugln("[s.getSchedule] ", "\nsql=", sql)

	id := -1
	//s.StartSecond = make([]time.Duration, 0)
	//循环读取记录，格式化后存入变量ｂ
	for rows.Next() {
		err = rows.Scan(&id, &s.Name, &s.Count, &s.Cyc,
			&s.TimeOut, &s.Desc, &s.CreateUserId, &s.CreateTime, &s.ModifyUserId, &s.ModifyTime)
		//s.setStart()
		if err != nil {
			e := fmt.Sprintf("getSchedule error %s\n", err.Error())
			return errors.New(e)
		}

	}

	if id == -1 {
		e := fmt.Sprintf("not found schedule [%d] from db.\n", s.Id)
		err = errors.New(e)
	}

	s.Jobs = make([]*Job, 0)
	s.Tasks = make([]*Task, 0)
	s.isRefresh = make(chan bool)
	s.JobCnt, s.TaskCnt = 0, 0
	return err
} // }}}

func (s *Schedule) getJobs() error {
	//查询全部Job列表
	sql := `SELECT job.id,
			   job.job_name,
			   job.job_desc,
			   job.exec_type,
			   job.disabled,
			   job.prev_job_id,
			   job.next_job_id,
               job.create_user_id,
               job.create_time,
               job.modify_user_id,
               job.modify_time
			FROM scd_job job
			WHERE job.scd_id=?`
	rows, err := g.HiveConn.Query(sql, s.Id)
	if err != nil {
		e := fmt.Sprintf("[\nj.getJob] run Sql %s error %s", sql, err.Error())
		return errors.New(e)
	}
	g.L.Debugln("[getJob] ", "\nsql=", sql)

	//循环读取记录，格式化后存入变量ｂ
	for rows.Next() {
		j := &Job{}
		err = rows.Scan(&j.Id, &j.Name, &j.Desc, &j.ExecType, &j.Disabled, &j.PreJobId, &j.NextJobId, &j.CreateUserId, &j.CreateTime, &j.ModifyUserId, &j.ModifyTime)
		if err != nil {
			e := fmt.Sprintf("\n[getJob] %s.", err.Error())
			return errors.New(e)
		}

		//初始化Task内存
		j.Tasks = make(map[string]*Task)
		s.Jobs = append(s.Jobs, j)
	}

	if s.Id == 0 {
		j := &Job{ScheduleCyc: "mi", Name: "DefaultJob"}
		s.Jobs = append(s.Jobs, j)
		return nil
	}
	return err
}

//从元数据库获取Job信息。
func (j *Job) getJob() error { // {{{
	if j.Id == 0 {
		j.ScheduleCyc = "mi"
		j.Name = "DefaultJob"
		return nil
	}
	//查询全部Job列表
	sql := `SELECT job.job_id,
			   job.job_name,
			   job.job_desc,
			   job.exec_type,
			   job.disabled,
			   job.prev_job_id,
			   job.next_job_id,
               job.create_user_id,
               job.create_time,
               job.modify_user_id,
               job.modify_time
			FROM scd_job job
			WHERE job.id=?`
	rows, err := g.HiveConn.Query(sql, j.Id)
	if err != nil {
		e := fmt.Sprintf("[\nj.getJob] run Sql %s error %s", sql, err.Error())
		return errors.New(e)
	}
	g.L.Debugln("[getJob] ", "\nsql=", sql)

	id := -1
	//循环读取记录，格式化后存入变量ｂ
	for rows.Next() {
		err = rows.Scan(&id, &j.Name, &j.Desc, &j.ExecType, &j.Disabled, &j.PreJobId, &j.NextJobId, &j.CreateUserId, &j.CreateTime, &j.ModifyUserId, &j.ModifyTime)
		if err != nil {
			e := fmt.Sprintf("\n[getJob] %s.", err.Error())
			return errors.New(e)
		}

		//初始化Task内存
		j.Tasks = make(map[string]*Task)
	}

	if id == -1 {
		e := fmt.Sprintf("[getJob] job [%d] not found \n", id)
		err = errors.New(e)
	}

	return err
} // }}}

//增加作业信息至元数据库
func (j *Job) add() (err error) { // {{{

	j.Tasks = make(map[string]*Task)
	j.CreateTime, j.ModifyTime = NowTimePtr(), NowTimePtr()
	sql := `INSERT INTO scd_job
            (job_name, job_desc, prev_job_id,
             next_job_id, create_user_id, create_time,
             modify_user_id, modify_time)
		VALUES      (?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := g.HiveConn.Exec(sql, &j.Name, &j.Desc, &j.PreJobId, &j.NextJobId, &j.CreateUserId, &j.CreateTime, &j.ModifyUserId, &j.ModifyTime)
	if err != nil {
		e := fmt.Sprintf("[j.add] run Sql error %s %s\n", sql, err.Error())
		return errors.New(e)
	}
	id, _ := result.LastInsertId()
	j.Id = id
	g.L.Debugln("[j.add] ", "\nsql=", sql)
	return err
} // }}}

//从元数据库获取Job下的Task列表。
func (j *Job) getTasksId() ([]int64, error) { // {{{
	tasksid := make([]int64, 0)

	//查询Job中全部Task列表
	sql := `SELECT id
			FROM scd_task
            WHERE job_id=?`
	rows, err := g.HiveConn.Query(sql, &j.Id)
	if err != nil {
		e := fmt.Sprintf("[j.getTasksId] Query sql [%s] error %s.\n", sql, err.Error())
		return tasksid, errors.New(e)
	}
	g.L.Debugln("[j.getTasksId] ", "\nsql=", sql)

	//循环读取记录
	for rows.Next() {
		var tid int64
		err = rows.Scan(&tid)
		tasksid = append(tasksid, tid)
	}
	return tasksid, err
} // }}}

//修改作业信息至元数据库
func (j *Job) update() (err error) { // {{{
	sql := `UPDATE scd_job
		SET job_name=?, 
			job_desc=?,
			prev_job_id=?,
            next_job_id=?, 
            modify_user_id=?, 
			modify_time=?
	    WHERE job_id=?`
	_, err = g.HiveConn.Exec(sql, &j.Name, &j.Desc, &j.PreJobId, &j.NextJobId, &j.ModifyUserId, &j.ModifyTime, &j.Id)
	if err != nil {
		e := fmt.Sprintf("[j.update] Query sql [%s] error %s.\n", sql, err.Error())
		err = errors.New(e)
	}
	return err
} // }}}

//删除作业信息至元数据库
func (j *Job) deleteJob() (err error) { // {{{
	sql := `DELETE FROM scd_job WHERE job_id=?`
	_, err = g.HiveConn.Exec(sql, &j.Id)
	if err != nil {
		e := fmt.Sprintf("[j.setNewId] Query sql [%s] error %s.\n", sql, err.Error())
		err = errors.New(e)
	}
	return err
} // }}}

//从元数据库获取Task信息。
func (t *Task) getTask() error { // {{{
	var td, id int64
	//查询全部Task列表
	sql := `SELECT task.id,
               task.task_address,
			   task.task_name,
			   task.task_time_out,
			   task.task_type,
			   task.task_cyc,
			   task.cronstr,
			   task.retry,
			   task.concurrent,
			   task.task_start,
			   task.disabled,
			   task.priority,
			   task.task_desc,
			   task.task_start,
			   task.task_cmd,
               task.create_user_id,
               task.create_time,
               task.modify_user_id,
               task.modify_time
			FROM scd_task task
			WHERE task.id=?`
	rows, err := g.HiveConn.Query(sql, t.Id)
	if err != nil {
		e := fmt.Sprintf("\n[t.getTask] sql %s error %s.", sql, err.Error())
		return errors.New(e)
	}

	//循环读取记录，格式化后存入变量ｂ
	for rows.Next() {
		err = rows.Scan(&id, &t.Address, &t.Name, &t.TimeOut, &t.TaskType, &t.TaskCyc, &t.Cronstr, &t.Retry, &t.Concurrent, &t.StartSecond, &t.Disabled, &t.Priority, &t.Desc, &td, &t.Cmd, &t.CreateUserId, &t.CreateTime, &t.ModifyUserId, &t.ModifyTime)
		if err != nil {
			e := fmt.Sprintf("\n[t.getTask] %s.", err.Error())
			return errors.New(e)
		}

		t.StartSecond = time.Duration(td) * time.Second
		//初始化relTask、param的内存
		t.RelTasksId = make([]int64, 0)
		t.RelTasks = make(map[string]*Task)
		//	t.Param = make([]string, 0)
		t.Attr = make(map[string]string)
	}

	if id == 0 {
		e := fmt.Sprintf("\n[t.getTask] task [%d] not found.", t.Id)
		err = errors.New(e)
	}

	return err
} // }}}

//从元数据库获取Job下的Task列表。
func (t *Task) getTaskAttr() error { // {{{

	//查询指定的Task属性列表
	sql := `SELECT ta.task_attr_name,
			   ta.task_attr_value
			FROM   scd_task_attr ta
			WHERE  task_id = ?`
	rows, err := g.HiveConn.Query(sql, t.Id)
	if err != nil {
		e := fmt.Sprintf("\n[t.getTaskAttr] sql %s error %s.", sql, err.Error())
		return errors.New(e)
	}

	//循环读取记录，格式化后存入变量ｂ
	for rows.Next() {
		var name, value string
		err = rows.Scan(&name, &value)
		if err != nil {
			e := fmt.Sprintf("\n[t.getTaskAttr] %s.", err.Error())
			return errors.New(e)
		}
		t.Attr[name] = value
	}
	return err
} // }}}

//从元数据库获取Task的依赖列表。
func (t *Task) getRelTaskId() error { // {{{
	//查询Task的依赖列表
	sql := `SELECT tr.rel_task_id
			FROM scd_task_rel tr
			Where tr.task_id=?`
	rows, err := g.HiveConn.Query(sql, t.Id)
	if err != nil {
		e := fmt.Sprintf("\n[t.getRelTaskId] sql %s error %s.", sql, err.Error())
		return errors.New(e)
	}

	//循环读取记录
	for rows.Next() {
		var rtid int64
		err = rows.Scan(&rtid)
		if err != nil {
			e := fmt.Sprintf("\n[t.getRelTaskId] %s.", err.Error())
			return errors.New(e)
		}
		t.RelTasksId = append(t.RelTasksId, rtid)
	}
	return err
} // }}}

//更新任务至元数据库
func (t *Task) update() error { // {{{
	sql := `UPDATE scd_task
			SET task_address=?,
				task_name=?,
				task_cyc=?,
				cronstr=?,
				retry=?,
				concurrent=?,
				task_time_out=?,
				task_start=?,
				task_type=?,
				task_cmd=?,
				task_desc=?,
				modify_user_id=?,
				modify_time=?
			WHERE id=?`
	_, err := g.HiveConn.Exec(sql, &t.Address, &t.Name, &t.TaskCyc, &t.Cronstr, &t.Retry, &t.Concurrent, &t.TimeOut, &t.StartSecond, &t.TaskType, &t.Cmd, &t.Desc, &t.ModifyUserId, &t.ModifyTime, &t.Id)
	if err != nil {
		e := fmt.Sprintf("\n[t.update] sql %s error %s.", sql, err.Error())
		return errors.New(e)
	}
	return err
} // }}}

//获取新JobTaskId
func (t *Task) getNewRelTaskId() (int64, error) { // {{{

	//查询全部schedule列表
	sql := `SELECT ifnull(max(rt.task_rel_id),0) as task_rel_id
			FROM scd_task_rel rt`

	rows, err := g.HiveConn.Query(sql)
	if err != nil {
		e := fmt.Sprintf("\n[t.getNewRelTaskId] sql %s error %s.", sql, err.Error())
		return -1, errors.New(e)
	}

	var id int64
	//循环读取记录，格式化后存入变量ｂ
	for rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			e := fmt.Sprintf("[t.getNewRelTaskId] %s.\n", err.Error())
			return -1, errors.New(e)
		}
	}

	return id + 1, err
} // }}}

//增加作业信息至元数据库
func (t *Task) add() (err error) { // {{{

	sql := `INSERT INTO scd_task
            (task_address, task_name, job_id,task_cyc,cronstr,retry,concurrent,
             task_time_out, task_start, task_type,
             task_cmd, task_desc, create_user_id, create_time,
             modify_user_id, modify_time)
			VALUES      (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,?)`
	result, err := g.HiveConn.Exec(sql, &t.Address, &t.Name, &t.JobId, &t.TaskCyc, &t.Cronstr, &t.Retry, &t.Concurrent, &t.TimeOut, &t.StartSecond, &t.TaskType, &t.Cmd, &t.Desc, &t.CreateUserId, &t.CreateTime, &t.ModifyUserId, &t.ModifyTime)
	if err != nil {
		e := fmt.Sprintf("\n[t.add] sql %s error %s.", sql, err.Error())
		return errors.New(e)
	}

	id, _ := result.LastInsertId()
	t.Id = id
	return err
} // }}}

//增加依赖任务至元数据库
func (t *Task) addRelTask(id int64) error { // {{{
	tm := time.Now()
	//relid, _ := t.getNewRelTaskId()
	sql := `INSERT INTO scd_task_rel
            (task_id, rel_task_id, create_user_id, create_time)
			VALUES      (?, ?, ?, ? )`
	_, err := g.HiveConn.Exec(sql, &t.Id, &id, &t.CreateUserId, &tm)
	if err != nil {
		e := fmt.Sprintf("\n[t.addRelTask] sql %s error %s.", sql, err.Error())
		return errors.New(e)
	}

	return err
} // }}}

//GetRelJobId获取最大的Id
func (t *Task) getRelJobId() (int64, error) { // {{{

	//查询全部schedule列表
	sql := `SELECT ifnull(max(t.job_task_id),0) as job_task_id
			FROM scd_job_task t`
	rows, err := g.HiveConn.Query(sql)
	if err != nil {
		e := fmt.Sprintf("\n[t.getRelJobId] sql %s error %s.", sql, err.Error())
		return -1, errors.New(e)
	}

	var id int64
	//循环读取记录，格式化后存入变量ｂ
	for rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			e := fmt.Sprintf("[t.getRelJobId] %s.\n", err.Error())
			return -1, errors.New(e)
		}
	}

	return id + 1, err
} // }}}

//删除依赖任务至元数据库
func (t *Task) deleteRelTask(id int64) error { // {{{
	sql := `DELETE FROM scd_task_rel WHERE task_id=? and rel_task_id=?`
	_, err := g.HiveConn.Exec(sql, &t.Id, &id)
	if err != nil {
		e := fmt.Sprintf("\n[t.deleteRelTask] sql %s error %s.", sql, err.Error())
		return errors.New(e)
	}

	return err
} // }}}

//删除任务至元数据库
func (t *Task) deleteTask() error { // {{{
	sql := `DELETE FROM scd_task WHERE id=?`
	_, err := g.HiveConn.Exec(sql, &t.Id)
	if err != nil {
		e := fmt.Sprintf("\n[t.deleteTask] sql %s error %s.", sql, err.Error())
		return errors.New(e)
	}

	return err
} // }}}

//保存执行日志
func (s *ExecSchedule) Log() (err error) { // {{{

	if s.schedule.Id == 0 {
		return nil
	}
	if s.state == 0 {
		sql := `INSERT INTO scd_schedule_log
						(batch_id,
						 scd_id,
						 start_time,
						 end_time,
						 state,
						 result,
						 batch_type)
			VALUES      (?,
						 ?,
						 ?,
						 ?,
						 ?,
						 ?,
						 ?)`
		result, err := g.LogConn.Exec(sql, &s.batchId, &s.schedule.Id, &s.startTime, &s.endTime, &s.state, &s.result, &s.execType)
		if err == nil {
			Id, _ := result.LastInsertId()
			s.LogId = int(Id)
		}
		return err
	} else {
		sql := `UPDATE scd_schedule_log
						 set start_time=?,
						 end_time=?,
						 state=?,
						 result=?
				WHERE log_id=?`
		_, err = g.LogConn.Exec(sql, &s.startTime, &s.endTime, &s.state, &s.result, &s.LogId)
	}
	return err
} // }}}

//保存执行日志
func (j *ExecJob) Log() (err error) { // {{{
	if j.job.Id == 0 {
		return nil
	}
	if j.state == 0 {
		sql := `INSERT INTO scd_job_log
						(batch_job_id,batch_id,
						 job_id,
						 start_time,
						 end_time,
						 state,
						 result,
						 batch_type)
			VALUES      (?,
						 ?,
						 ?,
						 ?,
						 ?,
						 ?,
						 ?,
						 ?)`
		result, err := g.LogConn.Exec(sql, &j.batchJobId, &j.batchId, &j.job.Id, &j.startTime, &j.endTime, &j.state, &j.result, &j.execType)
		if err == nil {
			Id, _ := result.LastInsertId()
			j.LogId = int(Id)
		}
		return err
	} else {
		sql := `UPDATE scd_job_log
						 set start_time=?,
						 end_time=?,
						 state=?,
						 result=?
				WHERE log_id=?`
		_, err = g.LogConn.Exec(sql, &j.startTime, &j.endTime, &j.state, &j.result, &j.LogId)
	}

	return err
} // }}}

//保存执行日志
func (t *ExecTask) Log() (err error) { // {{{
	if t.state == 0 {
		sql := `INSERT INTO scd_task_log
						(batch_task_id,batch_job_id,batch_id,
						 task_id,
						 start_time,
						 end_time,
						 state,
						 batch_type,
						 stdout,
						 stderr,
						 errmsg)
			VALUES      (?,
						 ?,
						 ?,
						 ?,
						 ?,
						 ?,
						 ?,
						 ?,
						 ?,
						 ?,
						 ?)`
		result, err := g.LogConn.Exec(sql, &t.batchTaskId, &t.batchJobId, &t.batchId, &t.task.Id, &t.startTime, &t.endTime, &t.state, &t.execType, &t.output, &t.stderr, &t.errstr)
		if err == nil {
			Id, _ := result.LastInsertId()
			t.LogId = int(Id)
		}
		return err
	} else {
		sql := `UPDATE scd_task_log
						 set start_time=?,
						 end_time=?,
						 state=?,
						 stdout=?,
						 stderr=?,
						 errmsg=?
				WHERE log_id=?`
		_, err = g.LogConn.Exec(sql, &t.startTime, &t.endTime, &t.state, &t.output, &t.stderr, &t.errstr, &t.LogId)
	}

	return err
} // }}}

//getSuccessTaskId会根据传入的batchId从元数据库查找出执行成功的task
func getSuccessTaskId(batchId string) []int64 { // {{{

	sql := `SELECT task_id
			FROM   scd_task_log
			WHERE  state = 3
			   AND batch_id =?`
	rows, err := g.HiveConn.Query(sql, batchId)
	CheckErr("getSuccessTaskId run Sql "+sql, err)

	taskIds := make([]int64, 0)
	//循环读取记录，格式化后存入变量ｂ
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		taskIds = append(taskIds, id)
	}

	return taskIds
} // }}}// }}}
