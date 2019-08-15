package strategy

import (
	"common/utils"
	"errors"
)

type roundRobin struct {
	lastServerIdx int       //上次获取到的服务器索引
	servers       []*server //服务器列表
	currentWeight int       //当前权重
}

//输入的对象指针
func NewRoundRobin(inputServers []*server) *roundRobin {

	defer utils.DealPanic()
	return &roundRobin{servers: inputServers, lastServerIdx: -1}
}

//获取后端服务器中最大的权重值
func (rr *roundRobin) getMaxWeight() int {

	defer utils.DealPanic()
	if len(rr.servers) == 0 {
		return 0
	}

	maxWeight := 0
	for _, s := range rr.servers {
		//当服务区down掉，或者CPU标高，都暂时跳过此服务器
		if !checkSeverState(s) {
			continue
		}

		if s.weight > maxWeight {
			maxWeight = s.weight
		}
	}

	return maxWeight
}

//辗转相除法：最大公约数
func gcdx(x, y int) int {

	defer utils.DealPanic()
	var tmp int
	for {
		tmp = (x % y)
		if tmp > 0 {
			x = y
			y = tmp
		} else {
			return y
		}
	}
}

func (rr *roundRobin) getMaxGcd() int {

	defer utils.DealPanic()
	maxGcd := maxWeight

	for _, s := range rr.servers {
		if !checkSeverState(s) {
			continue
		}
		maxGcd = gcdx(maxGcd, s.weight)
	}

	return maxGcd
}

//获取后端服务器
func (rr *roundRobin) getBackendServer() (*server, error) {

	defer utils.DealPanic()
	if len(rr.servers) == 0 {
		utils.Log.Debug(" no server")
		return nil, errors.New("no server")
	}

	for {
		rr.lastServerIdx = (rr.lastServerIdx + 1) % len(rr.servers)
		utils.Log.Debug("getBackendServer ServerIdx indx:=%d,len(rr.servers)=%d ", rr.lastServerIdx, len(rr.servers))
		if rr.lastServerIdx == 0 {
			rr.currentWeight = rr.currentWeight - rr.getMaxGcd()
			utils.Log.Debug("getBackendServer rr.getMaxGcd():%d,%d", rr.getMaxGcd(), rr.currentWeight)
			if rr.currentWeight <= 0 {
				rr.currentWeight = rr.getMaxWeight()
				utils.Log.Debug("max weight:%d", rr.currentWeight)
				if rr.currentWeight == 0 {
					return nil, errors.New("no server")
				}
			}
		}

		//当服务区down掉，或者CPU标高，都暂时跳过此服务器
		if !checkSeverState(rr.servers[rr.lastServerIdx]) {
			utils.Log.Debug("server %v state fail", rr.servers[rr.lastServerIdx].state.Down)
			continue
		}

		utils.Log.Debug("w:%d ,currentWeight:%d", rr.servers[rr.lastServerIdx].weight, rr.currentWeight)
		if rr.servers[rr.lastServerIdx].weight >= rr.currentWeight {
			return rr.servers[rr.lastServerIdx], nil
		}
	}
}

func checkSeverState(s *server) bool {
	defer utils.DealPanic()
	if s.state.Down == true || s.state.CupUtil > 90 {
		return false
	}

	return true
}
