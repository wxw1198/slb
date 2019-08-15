/*
Navicat MySQL Data Transfer

Source Server         : 101.132.99.183
Source Server Version : 50638
Source Host           : 101.132.99.183:3306
Source Database       : otvcloud

Target Server Type    : MYSQL
Target Server Version : 50638
File Encoding         : 65001

Date: 2018-01-24 20:30:04
*/

SET FOREIGN_KEY_CHECKS=0;

-- ----------------------------
-- Table structure for `slb_policy`
-- ----------------------------
DROP TABLE IF EXISTS `slb_policy`;
CREATE TABLE `slb_policy` (
  `UserID` varchar(255) NOT NULL,
  `Priority` int(11) DEFAULT NULL,
  `Ip` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`UserID`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

-- ----------------------------
-- Records of slb_policy
-- ----------------------------
INSERT INTO `slb_policy` VALUES ('1111', '6', '3333');
INSERT INTO `slb_policy` VALUES ('123456', '5', '123456');
INSERT INTO `slb_policy` VALUES ('2222', '22', '2222');
INSERT INTO `slb_policy` VALUES ('321', '3', '44');
