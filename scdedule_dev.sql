/*
SQLyog Community v12.09 (64 bit)
MySQL - 5.7.17-0ubuntu0.16.04.1 : Database - schedule_dev
*********************************************************************
*/

/*!40101 SET NAMES utf8 */;

/*!40101 SET SQL_MODE=''*/;

/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;
CREATE DATABASE /*!32312 IF NOT EXISTS*/`schedule_dev` /*!40100 DEFAULT CHARACTER SET utf8 COLLATE utf8_bin */;

USE `schedule_dev`;

/*Table structure for table `scd_job` */

DROP TABLE IF EXISTS `scd_job`;

CREATE TABLE `scd_job` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '调度id',
  `scd_id` bigint(20) NOT NULL,
  `job_name` varchar(256) NOT NULL COMMENT '作业名称',
  `job_desc` varchar(500) DEFAULT NULL COMMENT '作业说明',
  `prev_job_id` bigint(20) NOT NULL COMMENT '上级作业id',
  `next_job_id` bigint(20) NOT NULL COMMENT '下级作业id',
  `exec_type` tinyint(4) NOT NULL DEFAULT '0',
  `disabled` tinyint(4) NOT NULL DEFAULT '0',
  `create_user_id` bigint(20) DEFAULT NULL COMMENT '创建人',
  `create_time` datetime DEFAULT NULL COMMENT '创建时间',
  `modify_user_id` bigint(20) DEFAULT NULL COMMENT '修改人',
  `modify_time` datetime DEFAULT NULL COMMENT '修改时间',
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='作业信息：\n           调度部分，记录调度作业信息。';

/*Data for the table `scd_job` */

/*Table structure for table `scd_job_log` */

DROP TABLE IF EXISTS `scd_job_log`;

CREATE TABLE `scd_job_log` (
  `log_id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `job_id` bigint(20) NOT NULL COMMENT '作业id',
  `batch_job_id` varchar(128) NOT NULL COMMENT '作业批次id，规则 批次id+作业id',
  `batch_id` varchar(128) NOT NULL COMMENT '批次ID，规则scheduleId + 周期开始时间(不含周期内启动时间)',
  `start_time` datetime DEFAULT NULL COMMENT '开始时间',
  `end_time` datetime DEFAULT NULL COMMENT '结束时间',
  `state` varchar(1) DEFAULT NULL COMMENT '状态 0.不满足条件未执行 1. 执行中 2. 暂停 3. 完成 4.意外中止',
  `result` decimal(10,2) DEFAULT NULL COMMENT '结果,作业中执行成功任务的百分比',
  `batch_type` varchar(1) NOT NULL COMMENT '执行类型 1. 自动定时调度 2.手动人工调度 3.修复执行',
  PRIMARY KEY (`log_id`),
  KEY `job_id` (`job_id`,`batch_job_id`,`batch_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='作业执行信息表：\n           日志部分，记录作业执行情况。';

/*Data for the table `scd_job_log` */

/*Table structure for table `scd_schedule` */

DROP TABLE IF EXISTS `scd_schedule`;

CREATE TABLE `scd_schedule` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '调度id',
  `scd_name` varchar(256) NOT NULL COMMENT '调度名称',
  `scd_num` int(11) NOT NULL COMMENT '调度次数 0.不限次数 ',
  `scd_cyc` varchar(2) NOT NULL COMMENT '调度周期 ss 秒 mi 分钟 h 小时 d 日 m 月 w 周 q 季度 y 年',
  `scd_timeout` bigint(20) DEFAULT NULL COMMENT '最大执行时间，单位 秒',
  `scd_job_id` bigint(20) DEFAULT NULL COMMENT '作业id',
  `scd_desc` varchar(500) DEFAULT NULL COMMENT '调度说明',
  `create_user_id` varchar(30) NOT NULL COMMENT '创建人',
  `create_time` date NOT NULL COMMENT '创建时间',
  `modify_user_id` varchar(30) DEFAULT NULL COMMENT '修改人',
  `modify_time` date DEFAULT NULL COMMENT '修改时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='调度信息：\n           调度部分，记录调度信息。';

/*Data for the table `scd_schedule` */

/*Table structure for table `scd_schedule_log` */

DROP TABLE IF EXISTS `scd_schedule_log`;

CREATE TABLE `scd_schedule_log` (
  `log_id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `scd_id` bigint(20) NOT NULL COMMENT '调度id',
  `batch_id` varchar(128) NOT NULL COMMENT '批次ID，规则scheduleId + 周期开始时间(不含周期内启动时间)',
  `start_time` datetime DEFAULT NULL COMMENT '开始时间',
  `end_time` datetime DEFAULT NULL COMMENT '结束时间',
  `state` varchar(1) DEFAULT NULL COMMENT '状态 0.不满足条件未执行 1. 执行中 2. 暂停 3. 完成 4.失败',
  `result` decimal(10,2) DEFAULT NULL COMMENT '结果,调度中执行成功任务的百分比',
  `batch_type` varchar(1) DEFAULT NULL COMMENT '执行类型 1. 自动定时调度 2.手动人工调度 3.修复执行',
  PRIMARY KEY (`log_id`),
  KEY `scd_batch_id` (`scd_id`,`batch_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='用户调度权限表：\n           日志部分，记录调度执行情况。';

/*Data for the table `scd_schedule_log` */

/*Table structure for table `scd_task` */

DROP TABLE IF EXISTS `scd_task`;

CREATE TABLE `scd_task` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '任务id',
  `job_id` bigint(20) NOT NULL,
  `task_address` varchar(256) NOT NULL COMMENT '任务地址',
  `task_name` varchar(256) NOT NULL COMMENT '任务名称',
  `task_type` tinyint(20) NOT NULL COMMENT '任务类型 1 定时任务 2 依赖任务 0 手动执行',
  `task_cyc` varchar(2) NOT NULL DEFAULT '' COMMENT '调度周期 ss 秒 mi 分钟 h 小时 d 日 m 月 w 周 q 季度 y 年',
  `cronstr` varchar(1024) NOT NULL COMMENT 'crontab格式字符串 * * * * * *',
  `retry` int(11) NOT NULL DEFAULT '0' COMMENT '重试次数',
  `concurrent` int(11) NOT NULL DEFAULT '1',
  `priority` smallint(6) NOT NULL DEFAULT '0',
  `disabled` tinyint(4) NOT NULL DEFAULT '0',
  `task_time_out` bigint(20) DEFAULT '0' COMMENT '超时时间',
  `task_start` bigint(20) DEFAULT NULL COMMENT '周期内启动时间，格式 mm-dd hh24:mi:ss，最大单位小于调度周期',
  `task_cmd` varchar(2048) NOT NULL COMMENT '任务命令行',
  `task_desc` varchar(500) DEFAULT NULL COMMENT '任务说明',
  `create_user_id` varchar(30) DEFAULT '' COMMENT '创建人',
  `create_time` datetime DEFAULT NULL COMMENT '创建时间',
  `modify_user_id` bigint(20) DEFAULT NULL COMMENT '修改人',
  `modify_time` datetime DEFAULT NULL COMMENT '修改时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=30 DEFAULT CHARSET=utf8 COMMENT='任务信息：\r           任务部分，任务信息记录需要执行的具体任务，以及执行方式。由用户录入。';

/*Data for the table `scd_task` */

insert  into `scd_task`(`id`,`job_id`,`task_address`,`task_name`,`task_type`,`task_cyc`,`cronstr`,`retry`,`concurrent`,`priority`,`disabled`,`task_time_out`,`task_start`,`task_cmd`,`task_desc`,`create_user_id`,`create_time`,`modify_user_id`,`modify_time`) values (23,0,'127.0.0.1','test_timer',1,'','* * * * * *',1,0,0,0,0,0,'echo  task_timer','','1','2017-03-20 15:45:33',32,'2017-03-20 21:29:22'),(24,0,'127.0.0.1','test_ref1',2,'','* * * * * *',1,0,0,0,0,0,'echo  task_ref1','','1','2017-03-20 15:59:13',32,'2017-03-20 15:59:29'),(25,0,'127.0.0.1','test_ref2',2,'','* * * * * *',1,0,0,0,0,0,'echo  task_ref2','','1','2017-03-20 16:08:44',1,'2017-03-20 16:08:44'),(26,0,'127.0.0.1','test_ref3',2,'','* * * * * *',1,0,0,0,0,0,'echo  task_ref3','','1','2017-03-20 16:10:03',32,'2017-03-20 16:10:18'),(27,0,'127.0.0.1','test_ref4',2,'','* * * * * *',1,0,0,0,0,0,'echo  task_ref4','','1','2017-03-20 16:24:56',1,'2017-03-20 16:24:56'),(28,0,'127.0.0.1','test_ref5',2,'','* * * * * *',1,0,0,0,0,0,'echo  task_ref5','','1','2017-03-20 16:32:44',1,'2017-03-20 16:32:44'),(29,0,'127.0.0.1','test_do_task',0,'','* * * * * *',1,0,0,0,0,0,'echo  task_dotask','','1','2017-03-20 16:33:33',32,'2017-03-20 16:33:56');

/*Table structure for table `scd_task_attr` */

DROP TABLE IF EXISTS `scd_task_attr`;

CREATE TABLE `scd_task_attr` (
  `task_attr_id` bigint(20) NOT NULL COMMENT '自增id',
  `task_id` bigint(20) NOT NULL COMMENT '任务id',
  `task_attr_name` varchar(500) NOT NULL COMMENT '任务属性名称',
  `task_attr_value` text COMMENT '任务属性值',
  `create_time` date NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`task_attr_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='任务属性表：\n           任务部分，记录具体任务的属性值。';

/*Data for the table `scd_task_attr` */

/*Table structure for table `scd_task_log` */

DROP TABLE IF EXISTS `scd_task_log`;

CREATE TABLE `scd_task_log` (
  `log_id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `task_id` bigint(20) NOT NULL COMMENT '任务id',
  `batch_task_id` varchar(128) NOT NULL COMMENT '任务批次id，规则作业批次id+任务id',
  `batch_job_id` varchar(128) NOT NULL COMMENT '作业批次id，规则 批次id+作业id',
  `batch_id` varchar(128) NOT NULL COMMENT '批次ID，规则scheduleId + 周期开始时间(不含周期内启动时间)',
  `start_time` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '开始时间',
  `end_time` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '结束时间',
  `state` varchar(1) DEFAULT NULL COMMENT '状态 0.初始状态 1. 执行中 2. 暂停 3. 完成 4.忽略 5.意外中止',
  `batch_type` varchar(1) NOT NULL COMMENT '执行类型 1. 自动定时调度 2.手动人工调度 3.修复执行',
  `stdout` text NOT NULL COMMENT '标准输出',
  `stderr` text NOT NULL COMMENT '标准输出（错误）',
  `errmsg` text NOT NULL COMMENT '调度错误信息',
  PRIMARY KEY (`log_id`),
  KEY `task_id` (`task_id`,`batch_task_id`,`batch_job_id`,`batch_id`)
) ENGINE=InnoDB AUTO_INCREMENT=331642 DEFAULT CHARSET=utf8 COMMENT='任务执行信息表：\n           日志部分，记录任务执行情况。';

/*Data for the table `scd_task_log` */

/*Table structure for table `scd_task_rel` */

DROP TABLE IF EXISTS `scd_task_rel`;

CREATE TABLE `scd_task_rel` (
  `task_rel_id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增id',
  `task_id` bigint(20) NOT NULL COMMENT '任务id',
  `rel_task_id` bigint(20) NOT NULL COMMENT '依赖的任务id',
  `create_user_id` varchar(30) NOT NULL COMMENT '创建人',
  `create_time` datetime NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`task_rel_id`)
) ENGINE=InnoDB AUTO_INCREMENT=10 DEFAULT CHARSET=utf8 COMMENT='任务依赖关系表：\n           记录任务之间依赖关系，也就是本作业中准备执行的任务与上级作业中任务的';

/*Data for the table `scd_task_rel` */

insert  into `scd_task_rel`(`task_rel_id`,`task_id`,`rel_task_id`,`create_user_id`,`create_time`) values (5,24,23,'1','2017-03-20 15:59:13'),(6,25,23,'1','2017-03-20 16:08:44'),(7,26,23,'1','2017-03-20 16:10:03'),(8,27,23,'1','2017-03-20 16:24:56'),(9,28,23,'1','2017-03-20 16:32:44');

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;
