package main

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/levigross/grequests"
)

type rpcRequest struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      string        `json:"id"`
}

type rpcResponse struct {
	Result interface{}      `json:"result"`
	Error  rpcResponseError `json:"error"`
	ID     string           `json:"id"`
}

type rpcResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func rpcCall(req *rpcRequest) (*rpcResponse, error) {
	response, err := grequests.Post(rpcAddr, &grequests.RequestOptions{
		JSON: req,
	})
	if err != nil {
		return nil, err
	}
	var resp rpcResponse
	if err = response.JSON(&resp); err != nil {
		return nil, err
	}
	log.Debug("get rpc response", "resp", resp)
	if resp.Error.Code != 0 {
		return nil, errors.New(resp.Error.Message)
	}
	// if req.ID != resp.ID {
	// 	return nil, errors.New("id not match")
	// }
	return &resp, nil
}

func getNewAddress() (string, error) {
	id := uuid.New()
	req := &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "getnewaddress",
		Params:  nil,
		ID:      id.String(),
	}
	resp, err := rpcCall(req)
	if err != nil {
		return "", err
	}
	if addr, ok := resp.Result.(string); ok {
		return addr, nil
	}
	return "", errors.New("can not convert")
}

type transaction struct {
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time `sql:"index"`
	Txid             string     `gorm:"primary_key"`
	Symbol           string
	Fee              string
	SendingAddress   string
	ReferenceAddress string
	Amount           string
	Confirmations    int
	BlockHash        string
	Type             string
}

func (w *transaction) update() {
	DB.Save(w)
}

func (w *transaction) delete() {
	DB.Delete(w)
}

func findAllWatchers() (w []transaction) {
	DB.Find(&w)
	return
}

func getUsdtTx(txid string) (tx *transaction, err error) {
	defer func() {
		if e := recover(); e != nil {
			tx = nil
			var ok bool
			err, ok = e.(error)
			if !ok {
				err = errors.New("could not convert")
			}
		}
	}()
	id := uuid.New()
	req := &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "omni_gettransaction",
		Params:  []interface{}{txid},
		ID:      id.String(),
	}
	resp, err := rpcCall(req)
	if err != nil {
		return
	}
	if m, ok := resp.Result.(map[string]interface{}); ok && m["ismine"].(bool) {
		if valid, ok := m["valid"]; ok {
			if !valid.(bool) {
				err = errors.New("usdt tx not valid")
				return
			}
		}
		var t string
		if validateAddress(m["sendingaddress"].(string))&&validateAddress(m["referenceaddress"].(string)) {
			t="30"
		} else if validateAddress(m["sendingaddress"].(string)) {
			t = "20"
		} else if validateAddress(m["referenceaddress"].(string)) {
			t = "10"
		} else {
			err = errors.New("could not found from and to address in wallet")
			return
		}
		//if validateAddress(m["sendingaddress"].(string)) {
		//	t = "20"
		//} else if validateAddress(m["referenceaddress"].(string)) {
		//	t = "10"
		//} else {
		//	err = errors.New("could not found from and to address in wallet")
		//	return
		//}
		//if validateAddress(m["sendingaddress"].(string))&&validateAddress(m["referenceaddress"].(string)){
		//	t="30"
		//}
		tx = &transaction{
			Txid:             m["txid"].(string),
			Fee:              m["fee"].(string),
			SendingAddress:   m["sendingaddress"].(string),
			ReferenceAddress: m["referenceaddress"].(string),
			Amount:           m["amount"].(string),
			Confirmations:    int(m["confirmations"].(float64)),
			Symbol:           "USDT",
			Type:             t,
		}
		if hash, ok := m["blockhash"].(string); ok {
			tx.BlockHash = hash
		}
		return
	}
	err = errors.New("could not conver")
	return
}

type getBtcTxResult struct {
	Amount        float64                  `json:"amount"`
	Fee           float64                  `json:"fee"`
	Confirmations int                      `json:"confirmations"`
	BlockNumber   int                      `json:"blockindex"`
	BlockHash     string                   `json:"blockhash"`
	Details       []map[string]interface{} `json:"details"`
}

func getBtcTx(txid string) (tx *transaction, err error) {
	defer func() {
		if e := recover(); e != nil {
			tx = nil
			var ok bool
			err, ok = e.(error)
			if !ok {
				err = errors.New("could not convert")
			}
		}
	}()
	id := uuid.New()
	req := &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "gettransaction",
		Params:  []interface{}{txid},
		ID:      id.String(),
	}
	resp, err := rpcCall(req)
	if err != nil {
		return
	}
	data, err := json.Marshal(resp.Result)
	if err != nil {
		return
	}
	var result getBtcTxResult
	err = json.Unmarshal(data, &result)
	if err != nil {
		return
	}
	tx = &transaction{
		Symbol:        "BTC",
		Txid:          txid,
		Fee:           strconv.FormatFloat(result.Fee, 'f', -1, 64),
		Amount:        strconv.FormatFloat(result.Amount, 'f', -1, 64),
		Confirmations: result.Confirmations,
		BlockHash:     result.BlockHash,
	}
	if result.Amount < 0 {
		tx.Type = "20"
		tx.Amount = strconv.FormatFloat(-result.Amount, 'f', -1, 64)
		return
	}
	tx.Type = "10"
	for _, detail := range result.Details {
		if detail["category"].(string) == "receive" {
			if addr, ok := detail["address"].(string); ok {
				tx.ReferenceAddress = addr
				if result.Amount!=0 {
					amount,ok:= detail["amount"].(float64)
					if ok{
						amount1:=strconv.FormatFloat(amount,'f',-1,64)
						tx.Amount=amount1
						return
					}
					return
				}

				return
			}
		}
	}
	return
}

func sendBtc(to, amount string) (string, error) {
	id := uuid.New()
	req := &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "sendtoaddress",
		Params:  []interface{}{to, amount},
		ID:      id.String(),
	}
	resp, err := rpcCall(req)
	if err != nil {
		return "", err
	}
	if txid, ok := resp.Result.(string); ok {
		return txid, nil
	}
	return "", errors.New("could not convert")
}

func sendUsdt(from, to, amount string) (string, error) {
	id := uuid.New()
	req := &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "omni_send",
		Params:  []interface{}{from, to, 31, amount},
		ID:      id.String(),
	}
	resp, err := rpcCall(req)
	if err != nil {
		return "", err
	}
	if txid, ok := resp.Result.(string); ok {
		return txid, nil
	}
	return "", errors.New("could not convert")
}

type unspentStruct struct {
	Txid         string  `json:"txid"`
	Vout         int     `json:"vout"`
	Amount       float64 `json:"amount"`
	ScriptPubKey string  `json:"scriptPubKey"`
}

var makeTxLock sync.Mutex

func sendUsdtV2(from, to, amount, fee string) (string, error) {
	makeTxLock.Lock()
	defer makeTxLock.Unlock()
	if fee == "" {
		fee = "0.0006"
	}
	id := uuid.New()
	req := &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "listunspent",
		Params:  []interface{}{0, 99999999, []string{from}},
		ID:      id.String(),
	}
	resp, err := rpcCall(req)
	if err != nil {
		return "", err
	}
	unspentsBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return "", err
	}
	var unspents []*unspentStruct
	err = json.Unmarshal(unspentsBytes, &unspents)
	if err != nil {
		return "", err
	}
	req = &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "omni_createpayload_simplesend",
		Params:  []interface{}{31, amount},
		ID:      id.String(),
	}
	resp, err = rpcCall(req)
	if err != nil {
		return "", err
	}
	payload, ok := resp.Result.(string)
	if !ok {
		return "", errors.New("createpayload convert failed")
	}
	req = &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "createrawtransaction",
		Params:  []interface{}{unspents, make(map[string]string)},
		ID:      id.String(),
	}
	resp, err = rpcCall(req)
	if err != nil {
		return "", err
	}
	rawTx, ok := resp.Result.(string)
	if !ok {
		return "", errors.New("createrawtransactin convert failed")
	}
	req = &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "omni_createrawtx_opreturn",
		Params:  []interface{}{rawTx, payload},
		ID:      id.String(),
	}
	resp, err = rpcCall(req)
	if err != nil {
		return "", err
	}
	rawTx, ok = resp.Result.(string)
	if !ok {
		return "", errors.New("create opreturn convert failed")
	}
	req = &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "omni_createrawtx_reference",
		Params:  []interface{}{rawTx, to},
		ID:      id.String(),
	}
	resp, err = rpcCall(req)
	if err != nil {
		return "", err
	}
	rawTx, ok = resp.Result.(string)
	if !ok {
		return "", errors.New("create reference convert failed")
	}
	m := make([]map[string]interface{}, 0)
	for _, unspent := range unspents {
		mp := make(map[string]interface{})
		mp["txid"] = unspent.Txid
		mp["vout"] = unspent.Vout
		mp["scriptPubKey"] = unspent.ScriptPubKey
		mp["value"] = unspent.Amount
		m = append(m, mp)
	}
	if err != nil {
		return "", err
	}
	req = &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "omni_createrawtx_change",
		Params:  []interface{}{rawTx, m, from, fee},
		ID:      id.String(),
	}
	resp, err = rpcCall(req)
	if err != nil {
		return "", err
	}
	rawTx, ok = resp.Result.(string)
	if !ok {
		return "", errors.New("create change convert failed")
	}
	req = &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "signrawtransaction",
		Params:  []interface{}{rawTx},
		ID:      id.String(),
	}
	resp, err = rpcCall(req)
	if err != nil {
		return "", err
	}
	rawData, err := json.Marshal(resp.Result)
	if err != nil {
		return "", err
	}
	rawResp := struct {
		Hex      string `json:"hex"`
		Complete bool   `json:"complete"`
	}{}
	err = json.Unmarshal(rawData, &rawResp)
	if err != nil {
		return "", err
	}
	req = &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "sendrawtransaction",
		Params:  []interface{}{rawResp.Hex},
		ID:      id.String(),
	}
	resp, err = rpcCall(req)
	if err != nil {
		return "", err
	}
	txid, ok := resp.Result.(string)
	if !ok {
		return "", errors.New("send raw failed")
	}
	return txid, nil
}

func fundedSendUsdt(from, to, amount, feeaddress string) (string, error) {
	id := uuid.New()
	req := &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "omni_funded_send",
		Params:  []interface{}{from, to, 31, amount, feeaddress},
		ID:      id.String(),
	}
	resp, err := rpcCall(req)
	if err != nil {
		return "", err
	}
	if txid, ok := resp.Result.(string); ok {
		return txid, nil
	}
	return "", errors.New("could not convert")
}

func validateAddress(addr string) bool {
	id := uuid.New()
	req := &rpcRequest{
		Jsonrpc: "1.0",
		Method:  "validateaddress",
		Params:  []interface{}{addr},
		ID:      id.String(),
	}
	resp, err := rpcCall(req)
	if err != nil {
		return false
	}
	if m, ok := resp.Result.(map[string]interface{}); ok {
		ismine, ok := m["ismine"].(bool)
		if !ok {
			return false
		}
		iswatchonly, ok := m["iswatchonly"].(bool)
		if !ok {
			return false
		}
		return ismine && !iswatchonly
	}
	return false
}
func getbtcblance() (float64,error){
	id:=uuid.New()
	req:=&rpcRequest{
		Jsonrpc: "1.0",
		Method:  "getblance",
		Params:  []interface{}{"*"},
		ID:      id.String(),
	}
	resp, err := rpcCall(req)
	if err != nil {
		return 0,err
	}
	btcblance,ok:=resp.Result.(float64)
	if !ok {
		return 0, errors.New("getbtcblance convert failed")
	}
	return btcblance,nil

}
type usdtblanceTX struct {
	Propertyid     	int						 `json:"propertyid"`
	Name    		string                   `json:"name"`
	Balance     	string                   `json:"balance"`
	Reserved     	string                   `json:"reserved"`
	Frozen     		string                   `json:"frozen"`
}
func getusdtblance()(string,error)  {
	id:=uuid.New()
	req:=&rpcRequest{
		Jsonrpc: "1.0",
		Method:  "omni_getwalletbalances",
		Params:  []interface{}{},
		ID:      id.String(),
	}
	resp, err := rpcCall(req)
	if err != nil {
		return "",err
	}
	usdtblanceTXbates, err := json.Marshal(resp.Result)
	if err != nil {
		return "",err
	}
	var usdtblanceTXs []*usdtblanceTX
	err = json.Unmarshal(usdtblanceTXbates, &usdtblanceTXs)
	if err != nil {
		return "", err
	}
	for _, usdtblanceTX := range usdtblanceTXs {
		if (usdtblanceTX.Propertyid==31) {
			return usdtblanceTX.Balance,nil
		}
	}
	return "",errors.New("can't convert usdtlance")

}

type btcAddr struct {
	Address    		string                   `json:"address"`
	Balance     	float64                   `json:"balance"`
	Account 		string						`json:"account"`
}

func getbtcblancebyaddr(addr string)(float64,error){
	id:=uuid.New()
	req:=&rpcRequest{
		Jsonrpc: "1.0",
		Method:  "listaddressgroupings",
		Params:  []interface{}{},
		ID:      id.String(),
	}
	resp, err := rpcCall(req)
	if err != nil {
		return 0,err
	}
	rawData, err := json.Marshal(resp.Result)
	if err != nil {
		return 0,err
	}
	str:=string(rawData)
	var  resp1  [][][]interface{}
	err =json.Unmarshal([]byte(str),&resp1)
	if err!=nil {
		return 0,err
	}
	list:=parse(resp1)
	fmt.Println(list)
	for _,v3:=range list {
		if v3.Address==addr {
			return v3.Balance,nil
		}
	}
	return 0,errors.New("can't find your addr")
}

func parse(value [][][]interface{}) []btcAddr{
	accountSet:=[]btcAddr{}
	for _,items:=range value {
		for _,item:=range items{
			var ba  btcAddr
			length:=len(item)
			if length>0{
				address,ok:=item[0].(string)
				if !ok {
					continue
				}
				ba.Address=address
			}
			if length>1{
				blance,ok:=item[1].(float64)
				if !ok {
					continue
				}
				ba.Balance=blance
			}
			if length>2{
				account,ok:=item[2].(string)
				if !ok {
					continue
				}
				ba.Account=account
			}
			accountSet=append(accountSet,ba)
		}
	}
	return accountSet
}

//btc归集
func btcNotionalPooling()(string,error)  {
	btcblance1,err:=getbtcblance()
	if err!=nil {
		return "",err
	}
	btcblance:=strconv.FormatFloat(btcblance1-0.02,'f',-1,64)
	txid,err:=sendBtc(guijiaccout,btcblance)
	if err!=nil {
		log.Error("BTC归集失败","errors",err)
		return "",err
	}
	return txid,nil
}

//usdt归集
func usdtNotionalPooling(tx *transaction)(string,error)  {
	bol:=validateAddress(tx.ReferenceAddress)
	if bol{
		txid,err:=fundedSendUsdt(tx.ReferenceAddress,guijiaccout,tx.Amount,guijiaccout)
		if err!=nil {
			return "",err
		}
		return txid,nil
	}
	return "",nil
}

//type Spider struct {
//	url    string
//	header map[string]string
//}
//func (keyword Spider) get_html_header() string {
//	client := &http.Client{}
//	req, err := http.NewRequest("GET", keyword.url, nil)
//	if err != nil {
//		fmt.Println("1")
//	}
//	for key, value := range keyword.header {
//		req.Header.Add(key, value)
//	}
//	resp, err := client.Do(req)
//	if err != nil {
//		fmt.Println("2")
//	}
//	defer resp.Body.Close()
//	body, err := ioutil.ReadAll(resp.Body)
//	if err != nil {
//		fmt.Println("3")
//	}
//	return string(body)
//
//}
//func getbtcblancebyaddr(addr string) (string,error){
//		fmt.Println(addr)
//
//		header := map[string]string{
//			"Host": "tokenview.com",
//			"Connection": "keep-alive",
//			"Cache-Control": "max-age=0",
//			"Upgrade-Insecure-Requests": "1",
//			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/67.0.3396.99 Safari/537.36",
//			"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
//			"Referer": "https://btc.tokenview.com/cn/",
//			"If-None-Match": "e292-iJNnxSmtVxo8Rk6bFLkggWjDPwc",
//		}
//
//		url:="https://tokenview.com/cn/search/"+addr
//		spider := &Spider{url, header}
//		html := spider.get_html_header()
//		//比特币余额
//		pattern2:=`<div class="col"><span class="type">余额</span><span class="value">
//            (.*?)
//          </span></div>`
//		rp2 := regexp.MustCompile(pattern2)
//		find_txt2 := rp2.FindAllStringSubmatch(html,-1)
//		if find_txt2==nil{
//			return "",errors.New("can't convert btclance")
//		}
//		if find_txt2[0][1]=="" {
//			return "",errors.New("can't convert btclance")
//		}
//		fmt.Printf("%s\n",find_txt2[0][1])
//		return find_txt2[0][1],nil
//
//
//
//}