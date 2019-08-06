package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"

	zmq "github.com/go-zeromq/zmq4"
	"github.com/inconshreveable/log15"
	"github.com/json-iterator/go"
)

var (
	log     = log15.New()
	rpcAddr string
	debug   bool
	zmqAddr = "tcp://127.0.0.1:28332"
	json    = jsoniter.ConfigCompatibleWithStandardLibrary
	guijiaccout string
)

func main() {
	defer DB.Close()
	flag.BoolVar(&debug, "debug", false, "print debug info")
	flag.Parse()
	if debug {
		log.Info("running in debug")
	} else {
		handler := log.GetHandler()
		handler = log15.LvlFilterHandler(log15.LvlInfo, handler)
		log.SetHandler(handler)
		gin.SetMode(gin.ReleaseMode)
		//log.SetHandler(log15.MultiHandler(
		//	log15.StreamHandler(os.Stderr, log15.LogfmtFormat()),
		//	log15.LvlFilterHandler(
		//		log15.LvlInfo,
		//		log15.Must.FileHandler("errors.json", log15.JsonFormat()))))
	}
	//f, _ := os.Create("gin.log")
	//gin.DefaultWriter = io.MultiWriter(f)
	log.Info("load config done", "config", config)
	serverAddr := fmt.Sprintf(":%d", config.Port)
	rpcAddr = fmt.Sprintf("http://%s:%s@127.0.0.1:%d", config.RPCUser, config.RPCPassword, config.RPCPort)
	guijiaccout=config.GuiJiaccount
	app := gin.Default()
	app.Use(allowIPs)
	notify := app.Group("/notify")
	notify.GET("/block", notifyBlock)
	notify.GET("/tx/:txid", notifyTx)
	apiV1 := app.Group("/v1")
	apiV1.GET("/btc/address", getAddrCtrller)
	apiV1.POST("/btc/transaction", sendBtcCtrller)
	apiV1.GET("/usdt/address", getAddrCtrller)
	apiV1.POST("/usdt/transaction", sendUsdtCtrller)
	apiV1.POST("/btc/getAddrBalance",getAddrBalanceCll)
	apiV1.GET("/btc/btcNotionalPooling",btcNotionalPoolingCll)
	apiv2:=app.Group("/v2")
	//apiv2.GET("/btc/findbtc/:address",findBtcByAddrCll)
	apiv2.GET("/btc/btcbalance",getBtcBlanceCll)
	apiv2.POST("/btc/transaction",sendBtcCll)
	apiv2.GET("/usdt/usdtbalance",getUsdtBlanceCll)
	app.GET("/db", func(c *gin.Context) {
		var all []transaction
		DB.Find(&all)
		c.JSON(200, all)
		return
	})
	log.Info("start web server", "addr", serverAddr)
	if err := app.Run(serverAddr); err != nil {
		log.Crit("web server failed", "error", err)
	}
}

func subBlock() {
	blockSub := zmq.NewSub(context.Background())
	defer blockSub.Close()
	if err := blockSub.Dial(zmqAddr); err != nil {
		log.Crit("could not connect zmq", "addr", zmqAddr, "error", err)
		os.Exit(1)
	}
	if err := blockSub.SetOption(zmq.OptionSubscribe, "hashblock"); err != nil {
		log.Crit("could not subscribe hashblock", "error", err)
		os.Exit(1)
	}
	for {
		msg, err := blockSub.Recv()
		if err != nil {
			log.Error("could not receive message", "error", err)
			os.Exit(1)
		}
		log.Info("received message", "msg", msg.Frames)
	}
}

func subTx() {
	txSub := zmq.NewSub(context.Background())
	defer txSub.Close()
	if err := txSub.Dial(zmqAddr); err != nil {
		log.Crit("can not connect zmq", "addr", zmqAddr, "error", err)
		os.Exit(1)
	}
	if err := txSub.SetOption(zmq.OptionSubscribe, "rawtx"); err != nil {
		log.Crit("can not subcribe rawtx", "error", err)
		os.Exit(1)
	}
	for {
		msg, err := txSub.Recv()
		if err != nil {
			log.Error("could not receive message", "error", err)
			os.Exit(1)
		}
		log.Info("received message", "msg", msg.String())
	}
}
