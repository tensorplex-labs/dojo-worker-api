package sync

import (
	"dojo-api/pkg/blockchain"
	"dojo-api/pkg/orm"
	"github.com/rs/zerolog/log"
	"time"
)

func SyncDB() {
	subnetStateSubscriber := blockchain.GetSubnetStateSubscriberInstance()
	for {
		activeHotkeys := reverseMap(subnetStateSubscriber.SubnetState.ActiveMinerHotkeys)

		MinerUserORM := orm.NewMinerUserORM()

		dbHotkeys, err := MinerUserORM.GetMinerHotkeys()
		if err != nil {
			log.Error().Err(err).Msg("Error getting hotkeys from db")
		}

		for id , hotkey := range dbHotkeys {
			if _, ok := activeHotkeys[hotkey]; !ok {
				MinerUserORM.DeregisterMiner(id)
			}
		}

		time.Sleep(69 * blockchain.BlockTimeInSeconds * time.Second)
	}
}

func reverseMap(m map[int]string) map[string]int {
    n := make(map[string]int, len(m))
    for k, v := range m {
        n[v] = k
    }
    return n
}