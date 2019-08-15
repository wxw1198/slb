/*
Navicat MySQL Data Transfer

Source Server         : 101.132.99.183
Source Server Version : 50638
Source Host           : 101.132.99.183:3306
Source Database       : otvcloud

Target Server Type    : MYSQL
Target Server Version : 50638
File Encoding         : 65001

Date: 2018-01-27 19:56:55
*/

SET FOREIGN_KEY_CHECKS=0;

-- ----------------------------
-- Table structure for `slb_svr`
-- ----------------------------
DROP TABLE IF EXISTS `slb_svr`;
CREATE TABLE `slb_svr` (
  `serverid` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL COMMENT '服务器名称',
  `healthcheckport` int(11) NOT NULL,
  `ip` varchar(255) DEFAULT NULL,
  `weight` int(11) DEFAULT NULL,
  `specialty` varchar(255) DEFAULT NULL COMMENT '指定为cpu或gpu',
  `heartbeatInterval` int(11) DEFAULT NULL,
  `retryTime` int(11) DEFAULT NULL,
  `serverport` int(11) DEFAULT NULL,
  PRIMARY KEY (`serverid`)
) ENGINE=InnoDB AUTO_INCREMENT=102 DEFAULT CHARSET=utf8;

-- ----------------------------
-- Records of slb_svr
-- ----------------------------
INSERT INTO `slb_svr` VALUES ('1', 'server1', '80', '192.168.59.128', '6', 'cpu', '5000', '3', '100');
INSERT INTO `slb_svr` VALUES ('2', 'server2', '800', '192.168.59.129', '5', 'cpu', '5000', '3', '203');
