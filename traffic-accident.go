package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"

	pbase "github.com/synerex/synerex_proto"
	sxutil "github.com/synerex/synerex_sxutil"

	"log"
	"sync"
	"time"
)

type TrainStatus struct {
	ID     string `json:"id"`
	Step   string `json:"step"`
	AccFlg bool   `json:"acc_flg"`
}

var (
	nodesrv         = flag.String("nodesrv", "127.0.0.1:9990", "Node ID Server")
	local           = flag.String("local", "", "Local Synerex Server")
	mu              sync.Mutex
	version         = "0.0.0"
	role            = "TrafficAccident"
	sxServerAddress string
	accFlg          = false
	envClient       *sxutil.SXServiceClient
	accId           = "2"
	accStep         = "37"
)

func init() {
	flag.Parse()
}

func reconnectClient(client *sxutil.SXServiceClient) {
	mu.Lock()
	if client.SXClient != nil {
		client.SXClient = nil
		log.Printf("Client reset \n")
	}
	mu.Unlock()
	time.Sleep(5 * time.Second) // wait 5 seconds to reconnect
	mu.Lock()
	if client.SXClient == nil {
		newClt := sxutil.GrpcConnectServer(sxServerAddress)
		if newClt != nil {
			log.Printf("Reconnect server [%s]\n", sxServerAddress)
			client.SXClient = newClt
		}
	} else { // someone may connect!
		log.Printf("Use reconnected server [%s]\n", sxServerAddress)
	}
	mu.Unlock()
}

func resetHandler(w http.ResponseWriter, r *http.Request) {
	accId = r.URL.Query().Get("id")
	accStep = r.URL.Query().Get("step")
	log.Printf("Reset requested with id: %s and step: %s\n", accId, accStep)

	status := TrainStatus{ID: accId, Step: accStep}

	response, err := json.Marshal(status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func trainStatusHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	step := r.URL.Query().Get("step")
	log.Printf("Called /api/v0/train_status id: %s, step: %s\n", id, step)

	status := TrainStatus{ID: id, Step: step, AccFlg: false}
	smo := sxutil.SupplyOpts{
		Name: role,
		JSON: fmt.Sprintf(`{ "%s": null }`, role), // ここに事故情報を入れる
	}

	if id == accId && step == accStep {
		status.AccFlg = true
		t := time.Now()
		fmt.Println("事故発生時刻:", t.Format("15:04:05"))
		smo = sxutil.SupplyOpts{
			Name: role,
			JSON: fmt.Sprintf(`{ "%s": { "time": "18:00", "station": "犬山線布袋駅", "type": "人身事故" } }`, role), // ここに事故情報を入れる
		}
	}

	_, nerr := envClient.NotifySupply(&smo)
	if nerr != nil {
		log.Printf("Send Fail! %v\n", nerr)
	} else {
		//							log.Printf("Sent OK! %#v\n", ge)
	}

	response, err := json.Marshal(status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func main() {
	go sxutil.HandleSigInt()
	sxutil.RegisterDeferFunction(sxutil.UnRegisterNode)
	log.Printf("%s(%s) built %s sha1 %s", role, sxutil.GitVer, sxutil.BuildTime, sxutil.Sha1Ver)

	channelTypes := []uint32{pbase.JSON_DATA_SVC}

	var rerr error
	sxServerAddress, rerr = sxutil.RegisterNode(*nodesrv, role, channelTypes, nil)

	if rerr != nil {
		log.Fatal("Can't register node:", rerr)
	}
	if *local != "" { // quick hack for AWS local network
		sxServerAddress = *local
	}
	log.Printf("Connecting SynerexServer at [%s]", sxServerAddress)

	wg := sync.WaitGroup{} // for syncing other goroutines

	client := sxutil.GrpcConnectServer(sxServerAddress)

	if client == nil {
		log.Fatal("Can't connect Synerex Server")
	} else {
		log.Print("Connecting SynerexServer")
	}

	envClient = sxutil.NewSXServiceClient(client, pbase.JSON_DATA_SVC, fmt.Sprintf("{Client:%s}", role))

	http.HandleFunc("/api/v0/train_status", trainStatusHandler)
	http.HandleFunc("/api/v0/reset", resetHandler)
	fmt.Println("Server is running on port 8030")
	go http.ListenAndServe(":8030", nil)

	// trainStatusHandler に移植したためコメントアウト
	// // タイマーを開始する
	// ticker := time.NewTicker(15 * time.Second)
	// defer ticker.Stop()

	// // 現在時刻を取得し、次の実行時刻まで待機する
	// start := time.Now()
	// adjust := start.Truncate(15 * time.Second).Add(5 * time.Second)
	// time.Sleep(adjust.Sub(start))

	// i := 0
	// for {
	// 	select {
	// 	case t := <-ticker.C:
	// 		// ここに実行したい処理を書く
	// 		fmt.Println("実行時刻:", t.Format("15:04:05"))
	// 		smo := sxutil.SupplyOpts{
	// 			Name: role,
	// 			JSON: fmt.Sprintf(`{ "%s": null }`, role), // ここに事故情報を入れる
	// 		}
	// 		// if i%4 == 0 {
	// 		// 	smo.JSON = fmt.Sprintf(`{ "%s": { "time": "18:00", "station": "犬山線布袋駅", "type": "人身事故" } }`, role)
	// 		// }
	// 		_, nerr := envClient.NotifySupply(&smo)
	// 		if nerr != nil {
	// 			log.Printf("Send Fail! %v\n", nerr)
	// 		} else {
	// 			//							log.Printf("Sent OK! %#v\n", ge)
	// 		}
	// 		i++
	// 	}
	// }

	wg.Add(1)
	// log.Print("Subscribe Supply")
	// go subscribeJsonRecordSupply(envClient)
	wg.Wait()
}
