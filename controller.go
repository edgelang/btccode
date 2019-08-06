package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/levigross/grequests"
)

func writeError(c *gin.Context, status int, code string, msg string) {
	c.JSON(status, gin.H{
		"code":    code,
		"message": msg,
		"result":  nil,
	})
}

func allowIPs(c *gin.Context) {
	//ip白名单判断
	ip := c.ClientIP()
	if ip == "127.0.0.1" || ip == "::1" {
		c.Next()
		return
	}
	for _, allow := range config.AllowIP {
		if ip == allow {
			c.Next()
			return
		}
	}
	c.AbortWithStatus(401)
	return
}
//判断本地数据库里的数据的区块确认数是否够
func notifyBlock(c *gin.Context) {
	c.String(200, "success")
	watchers := findAllWatchers()
	for _, tx := range watchers {
		//判断币种类型
		if tx.Symbol == "USDT" {
			txn, err := getUsdtTx(tx.Txid)
			if txn.Type=="30"{
				txn.delete()
				//c.String(200, "success")
				continue
			}
			if err != nil {
				log.Error("get usdt tx in watch list failed", "error", err)
				continue
			}
			txn.update()
			if txn.Confirmations >= config.Confirmation {
				if err := postToServer(txn); err != nil {
					log.Error("post to server failed", "error", err)
				} else {
					log.Info("post to server success", "tx", txn)
					if txn.ReferenceAddress!=guijiaccout&&txn.Type=="10"{
						_,err:=usdtNotionalPooling(txn)
						if err!=nil{
							log.Error("usdt归集失败","erros",err)
						}
					}
					log.Info("usdt归集成功")
					txn.delete()
				}
			}
		} else if tx.Symbol == "BTC" {
			txn, err := getBtcTx(tx.Txid)
			if err != nil {
				log.Error("get usdt tx in watch list failed", "error", err)
				continue
			}
			txn.update()
			if txn.Confirmations >= config.Confirmation {
				if err := postToServer(txn); err != nil {
					log.Error("post to server failed", "error", err)
				} else {
					log.Info("post to server success", "tx", txn)
					txn.delete()
				}
			}
		}
	}
	return
}
//通过txid查询交易详细信息，保存在数据库里，如果区块确认数大于3
//传给服务器，服务器确认后删除这笔交易信息
func notifyTx(c *gin.Context) {
	txid := c.Param("txid")
	if txid == "" {
		c.String(400, "error")
		return
	}
	usdtTx, err := getUsdtTx(txid)
	if err == nil {
		if usdtTx.Type=="30"{
			usdtTx.delete()
			c.String(200, "success")
			return
		}
		usdtTx.update()
		if usdtTx.Confirmations >= config.Confirmation {
			if err := postToServer(usdtTx); err != nil {
				log.Error("post to server failed", "error", err)
				c.String(400, "error")
				return
			}
			log.Info("post to server success", "tx", usdtTx)
			if usdtTx.ReferenceAddress!=guijiaccout&&usdtTx.Type=="10"{
				_,err:=usdtNotionalPooling(usdtTx)
				if err!=nil{
					log.Error("usdt归集失败","erros",err)
				}
			}
			log.Info("usdt归集成功")
			usdtTx.delete()
		}
		c.String(200, "success")
		return
	}
	log.Debug("find usdt failed", "error", err)
	btcTx, err := getBtcTx(txid)
	if err != nil {
		log.Crit("notify tx not found in full node", "txid", txid, "error", err)
		c.String(400, "error")
		return
	}
	btcTx.update()
	if btcTx.Confirmations >= config.Confirmation {
		if err := postToServer(btcTx); err != nil {
			log.Error("post to server failed", "error", err)
			c.String(400, "error")
			return
		}
		log.Info("post to server success", "tx", btcTx)
		btcTx.delete()
	}
	c.String(200, "success")
	return
}

//得到一个新的BTC地址
func getAddrCtrller(c *gin.Context) {
	addr, err := getNewAddress()
	if err != nil {
		writeError(c, 500, "1", err.Error())
		return
	}
	c.JSON(200, gin.H{
		"code":    "0",
		"message": "success",
		"result":  addr,
	})
	return
}

type sendBtcRequest struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount string `json:"amount"`
	Symbol string `json:"symbol"`
}

//发送比特币给给定地址
func sendBtcCtrller(c *gin.Context) {
	var req sendBtcRequest
	req.From=c.PostForm("from")
	req.To=c.PostForm("to")
	req.Amount=c.PostForm("amount")
	req.Symbol=c.PostForm("symbol")
	log.Info("send btc", "request", req)
	if req.Symbol=="BTC" {
		if req.To == "" || req.Amount == "" {
			writeError(c, 400, "2", "to and amount required")
			return
		}
		txid, err := sendBtc(req.To, req.Amount)
		if err != nil {
			writeError(c, 500, "1", err.Error())
			return
		}
		c.JSON(200, gin.H{
			"code":    "0",
			"message": "success",
			"result":  txid,
		})
		return
	}
	if req.Symbol=="USDT" {
		if req.To == "" || req.Amount == "" || req.From == "" {
			writeError(c, 400, "2", "to and amount required")
			return
		}
		txid, err := sendUsdtV2(req.From, req.To, req.Amount, "")
		if err != nil {
			writeError(c, 500, "1", err.Error())
			return
		}
		c.JSON(200, gin.H{
			"code":    "0",
			"message": "success",
			"result":  txid,
		})
		return
	}
	writeError(c, 500, "1", "找不到代币类型")
	return
}

type sendUsdtRequest struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Amount     string `json:"amount"`
	FeeAddress string `json:"feeaddress"`
	Fee        string `json:"fee"`
}
//发送usdt
func sendUsdtCtrller(c *gin.Context) {
	//取得传过来的json数据
	//data, err := c.GetRawData()
	//if err != nil {
	//	writeError(c, 400, "2", "params invalid")
	//	return
	//}
	//reqBody := strings.Replace(string(data), `"`, "",10)
	//log.Info("send usdt", "request", reqBody)
	var req sendUsdtRequest
	//将json数据反序列化
	req.Fee=c.PostForm("fee")
	req.To=c.PostForm("to")
	req.From=c.PostForm("from")
	req.Amount=c.PostForm("amount")
	req.FeeAddress=c.PostForm("feeaddress")
	//if err := json.Unmarshal(data, &req); err != nil {
	//	writeError(c, 400, "2", "params invalid")
	//	return
	//}
	if req.To == "" || req.Amount == "" || req.From == "" {
		writeError(c, 400, "2", "to and amount required")
		return
	}
	var txid string
	var err error
	//判断是否有手续费的接收地址
	if req.FeeAddress == "" {
		txid, err = sendUsdtV2(req.From, req.To, req.Amount, req.Fee)
	} else {
		txid, err = fundedSendUsdt(req.From, req.To, req.Amount, req.FeeAddress)
	}
	if err != nil {
		writeError(c, 500, "1", err.Error())
		return
	}
	c.JSON(200, gin.H{
		"code":    "0",
		"message": "success",
		"result":  txid,
	})
	return
}

type antWalletRequest struct {
	Sign             string `json:"sign"`         //签名
	Amount           string `json:"amount"`		  //数量
	To 				 string `json:"to"`			  //到达地址
	From   			 string `json:"from"`		  //发送地址
	Txid             string `json:"txid"`		  //交易ID
	Type             string `json:"type"`         //交易类型10提现20充值
	Fee				 string `json:"fee"`          //手续费多少
	FeeSymbol        string `json:"feesymbol"`	  //手续费类型
	Symbol	   		 string `json:"symbol"`       //代币类型
}
//将交易数据发送给服务器
func postToServer(tx *transaction) error {
	req := &antWalletRequest{
		Amount:           tx.Amount,
		To: 			  tx.ReferenceAddress,
		From:   		  tx.SendingAddress,
		Txid:             tx.Txid,
		Type:             tx.Type,
		Fee:              tx.Fee,
		Symbol:           tx.Symbol,
		FeeSymbol:		  "BTC",
	}
	var signStr string
	signStr = fmt.Sprintf("amount=%s", req.Amount)
	if req.Fee!="0" {
		signStr+=fmt.Sprintf("&fee=%s", req.Fee)
	}
	signStr+=fmt.Sprintf("&fee_symbol=%s",req.FeeSymbol)
	if req.From != ""{
		signStr += "&from=" + req.From
	}
	signStr+=fmt.Sprintf("&symbol=%s", req.Symbol)
	if req.To != "" {
		signStr += "&to=" + req.To
	}
	signStr += fmt.Sprintf("&txid=%s&type=%s", req.Txid, req.Type)
	fmt.Println(signStr)
	log.Debug("sign", "string", signStr)
	//对数据进行加密处理
	mac := hmac.New(sha1.New, []byte("daxingxing_eth"))
	mac.Write([]byte(signStr))
	signHmac := mac.Sum(nil)
	req.Sign = base64.StdEncoding.EncodeToString(signHmac)
	log.Info("post to server", "request", req)
	personSalary := map[string]string {
		"amount": 	tx.Amount,
		"to":		tx.ReferenceAddress,
		"from":		tx.SendingAddress,
		"symbol":	tx.Symbol,
		"txid":		tx.Txid,
		"fee_symbol":"BTC",
		"type":		tx.Type,
		"sign":		base64.StdEncoding.EncodeToString(signHmac),
		"fee":		tx.Fee,
	}
	response, err := grequests.Post(config.Callback, &grequests.RequestOptions{
		Data:personSalary,
	})
	if err != nil {
		return err
	}
	respData := response.Bytes()
	log.Info("post to server", "response", string(respData))
	if response.StatusCode != 200 {
		return errors.New("status code not 200")
	}
	var resp struct {
		Data   string `json:"data"`
		Msg    string `json:"msg"`
		Status int    `json:"code"`
	}
	if err := json.Unmarshal(respData, &resp); err != nil {
		return err
	}
	if resp.Data != "success" {
		return fmt.Errorf("status: %v, msg: %s, data: %s", resp.Status, resp.Msg, resp.Data)
	}
	return nil
}
type GetBalanceRequest struct {
	Address	string
	Symbol  string
}
//通过地址查询比特币或者USDT余额
func getAddrBalanceCll(c *gin.Context) {
	var req GetBalanceRequest
	req.Address=c.PostForm("address")
	req.Symbol=c.PostForm("symbol")
	if req.Address==""||req.Symbol==""{
		writeError(c, 400, "2", "address and symbol required")
	}
	if req.Symbol=="BTC" {
		balance,err:=getbtcblancebyaddr(req.Address)
		if err != nil {
			writeError(c, 500, "1", err.Error())
			return
		}
		c.JSON(200, gin.H{
			"code":    "0",
			"message": "success",
			"result":  balance,
		})
		return
	}
	if req.Symbol=="USDT"{

	}
}
//得到节点的比特币余额
func getBtcBlanceCll(c *gin.Context) {

	balance,err:=getbtcblance()
	if err != nil {
		writeError(c, 500, "1", err.Error())
		return
	}
	c.JSON(200, gin.H{
		"code":    "0",
		"message": "success",
		"result":  balance,
	})
	return
}
func sendBtcCll(c *gin.Context) {
	data, err := c.GetRawData()
	if err != nil {
		writeError(c, 400, "4", "params invalid")
		return
	}
	//reqBody := strings.ReplaceAll(string(data), `"`, "")
	//log.Info("send btc", "request", reqBody)
	println(string(data))
	var req sendBtcRequest
	if err := json.Unmarshal(data, &req); err != nil {
		writeError(c, 400, "3", "params invalid")
		return
	}
	if req.To == "" || req.Amount == "" {
		writeError(c, 400, "2", "to and amount required")
		return
	}
	txid, err := sendBtc(req.To, req.Amount)
	if err != nil {
		writeError(c, 500, "1", err.Error())
		return
	}
	btcTx, err := getBtcTx(txid)
	if err != nil {
		writeError(c, 500, "2", err.Error())
	}

	c.JSON(200, gin.H{
		"code":    "0",
		"message": "success",
		"result":  btcTx,
	})
	return
}
func getUsdtBlanceCll(c *gin.Context)  {
	usdtbalance,err:=getusdtblance()
	if err != nil {
		writeError(c, 500, "1", err.Error())
		return
	}
	c.JSON(200, gin.H{
		"code":    "0",
		"message": "success",
		"result":  usdtbalance,
	})
	return
}

//BTC归集
func btcNotionalPoolingCll(c *gin.Context)  {
	txid,err:=btcNotionalPooling()
	if err != nil {
		writeError(c, 500, "1", err.Error())
		return
	}
	c.JSON(200, gin.H{
		"code":    "0",
		"message": "success",
		"result":  txid,
	})
	return
}
