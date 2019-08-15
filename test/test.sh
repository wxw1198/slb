#!/bin/sh


curl -i -d '{"UserID":"1234567","SessonID":"3333","TaskType":"cpu","ReqMode":"selectServer"}' http://10.80.3.173:8081/yfy/select/lb/server


curl -i -d '{"Ip":"192.168.1.251","CupUtil":91,"IoWait":20,"MemUtil":63, "MemTotal":125,"Down":false}' http://10.80.3.173:8081/yfy/server/state


curl -i -d '{"UserID":"123456","Priority":5,"Ip":"192.168.1.252"}' http://10.80.3.173:8081/yfy/user/policy


# http://ip:port/yfy/config/file
