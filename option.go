package requests

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/gospider007/tools"
	"github.com/gospider007/websocket"
	"github.com/quic-go/quic-go"
	uquic "github.com/refraction-networking/uquic"
	utls "github.com/refraction-networking/utls"
)

type LogType string

const (
	LogType_DNSLookup          LogType = "DNSLookup"
	LogType_TCPConnect         LogType = "TCPConnect"
	LogType_TLSHandshake       LogType = "TLSHandshake"
	LogType_ProxyDNSLookup     LogType = "ProxyDNSLookup"
	LogType_ProxyTCPConnect    LogType = "ProxyTCPConnect"
	LogType_ProxyTLSHandshake  LogType = "ProxyTLSHandshake"
	LogType_ProxyConnectRemote LogType = "ProxyConnectRemote"
	LogType_ResponseHeader     LogType = "ResponseHeader"
	LogType_ResponseBody       LogType = "ResponseBody"
)

type Log struct {
	Time time.Time
	Msg  any     `json:"msg"`
	Id   string  `json:"id"`
	Type LogType `json:"type"`
}

// Connection Management Options
type ClientOption struct {
	Spec                  string //goSpiderSpec   origin : https://github.com/gospider007/fp
	DialOption            DialOption
	Headers               any                              //default headers
	Jar                   Jar                              //custom cookies
	Logger                func(Log)                        //debuggable
	OptionCallBack        func(ctx *Response) error        //option callback,if error is returnd, break request
	ResultCallBack        func(ctx *Response) error        //result callback,if error is returnd,next errCallback
	ErrCallBack           func(ctx *Response) error        //error callback,if error is returnd,break request
	RequestCallBack       func(ctx *Response) error        //request and response callback,if error is returnd,reponse is error
	GetProxy              func(ctx *Response) (any, error) //proxy callback:support https,http,socks5 proxy
	TlsConfig             *tls.Config
	UtlsConfig            *utls.Config
	QuicConfig            *quic.Config
	UquicConfig           *uquic.Config
	USpec                 any           //support ja3.USpec,uquic.QUICID,bool
	UserAgent             string        //headers User-Agent value
	Proxy                 any           //strong or []string, ,support https,http,socks5
	MaxRetries            int           //try num
	MaxRedirect           int           //redirect num ,<0 no redirect,==0 no limit
	Timeout               time.Duration //request timeout
	ResponseHeaderTimeout time.Duration //ResponseHeaderTimeout ,default:300
	TlsHandshakeTimeout   time.Duration //tls timeout,default:15
	ForceHttp3            bool          //force  use http3 send requests
	ForceHttp1            bool          //force  use http1 send requests
	DisCookie             bool          //disable cookies
	DisDecode             bool          //disable auto decode
	Bar                   bool          ////enable bar display
}

// Options for sending requests
type RequestOption struct {
	WsOption websocket.Option //websocket option
	Cookies  any              // cookies,support :json,map,str，http.Header
	Params   any              //url params，join url query,json,map
	Json     any              //send application/json,support io.Reader,：string,[]bytes,json,map
	Data     any              //send application/x-www-form-urlencoded, support io.Reader, string,[]bytes,json,map
	Form     any              //send multipart/form-data,file upload,support io.Reader, json,map
	Text     any              //send text/xml,support: io.Reader, string,[]bytes,json,map
	Body     any              //not setting context-type,support io.Reader, string,[]bytes,json,map
	Url      *url.URL
	// other option
	Method      string //method
	Host        string
	Referer     string //set headers referer value
	ContentType string //headers Content-Type value
	ClientOption
	Stream       bool //disable auto read
	DisProxy     bool //force disable proxy
	once         bool
	orderHeaders *OrderData //order headers
	gospiderSpec *GospiderSpec
}

// Upload files with form-data,
type File struct {
	Content     any
	FileName    string
	ContentType string
}

func randomBoundary() string {
	var buf [30]byte
	io.ReadFull(rand.Reader, buf[:])
	boundary := fmt.Sprintf("%x", buf[:])
	if strings.ContainsAny(boundary, `()<>@,;:\"/[]?= `) {
		boundary = `"` + boundary + `"`
	}
	return "multipart/form-data; boundary=" + boundary
}

func (obj *RequestOption) initBody(ctx context.Context) (io.Reader, error) {
	if obj.Body != nil {
		body, orderData, _, err := obj.newBody(obj.Body)
		if err != nil {
			return nil, err
		}
		if body != nil {
			return body, nil
		}
		con, err := orderData.MarshalJSON()
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(con), nil
	} else if obj.Form != nil {
		if obj.ContentType == "" {
			obj.ContentType = randomBoundary()
		}
		body, orderData, ok, err := obj.newBody(obj.Form)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("not support type")
		}
		if body != nil {
			return body, nil
		}
		body, once, err := orderData.parseForm(ctx)
		if err != nil {
			return nil, err
		}
		obj.once = once
		return body, err
	} else if obj.Data != nil {
		if obj.ContentType == "" {
			obj.ContentType = "application/x-www-form-urlencoded"
		}
		body, orderData, ok, err := obj.newBody(obj.Data)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("not support type")
		}
		if body != nil {
			return body, nil
		}
		return orderData.parseData(), nil
	} else if obj.Json != nil {
		if obj.ContentType == "" {
			obj.ContentType = "application/json"
		}
		body, orderData, _, err := obj.newBody(obj.Json)
		if err != nil {
			return nil, err
		}
		if body != nil {
			return body, nil
		}
		return orderData.parseJson()
	} else if obj.Text != nil {
		if obj.ContentType == "" {
			obj.ContentType = "text/plain"
		}
		body, orderData, _, err := obj.newBody(obj.Text)
		if err != nil {
			return nil, err
		}
		if body != nil {
			return body, nil
		}
		return orderData.parseText()
	} else {
		return nil, nil
	}
}
func (obj *RequestOption) initParams() (*url.URL, error) {
	baseUrl := cloneUrl(obj.Url)
	if obj.Params == nil {
		return baseUrl, nil
	}
	body, dataData, ok, err := obj.newBody(obj.Params)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("not support type")
	}
	var query string
	if body != nil {
		paramsBytes, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		query = tools.BytesToString(paramsBytes)
	} else {
		query = dataData.parseParams().String()
	}
	if query == "" {
		return baseUrl, nil
	}
	pquery := baseUrl.Query().Encode()
	if pquery == "" {
		baseUrl.RawQuery = query
	} else {
		baseUrl.RawQuery = pquery + "&" + query
	}
	return baseUrl, nil
}
func (obj *Client) newRequestOption(option RequestOption) (RequestOption, error) {
	err := tools.Merge(&option, obj.ClientOption)
	//end
	if option.MaxRetries < 0 {
		option.MaxRetries = 0
	}
	if option.UserAgent == "" {
		option.UserAgent = obj.ClientOption.UserAgent
	}
	if option.DisCookie {
		option.Jar = nil
	}
	if option.DisProxy {
		option.Proxy = ""
	}
	if option.Spec == "" {
		option.Spec = "1603010700010006fc0303c4d7be11dc91da4dced4e4189f5ccfce7c4d05bfa760ffc50067426b7cf6d13c2045995420c14ef81f09398076f0cfc4c54fbd4cf39003a4c1d5eaf44b11e9107300207a7a130113021303c02bc02fc02cc030cca9cca8c013c014009c009d002f0035010006934a4a000000170000000a000c000afafa11ec001d0017001844cd00050003026832002d000201010010000e000c02683208687474702f312e31002b0007068a8a03040303001b000302000200230000000500050100000000000d0012001004030804040105030805050108060601000b00020100ff0100010000120000fe0d011a0000010001460020fc9d027aaffdd58b1dc3cca68dd6779d31e2a8c0e84e5e56199cf7a33b805e0200f05c0b63fe280a4c3f941669f883c5caeff4cbf9cf772a70355698f884f4aa40827a00862abd92dff29c79d6fd74dbf9af4eff641a61039b298ecaf1aebd5383972aa221593ea6c05627552e2302a95e8a4dc689bc6a63defcc76e864962cb287475884bfcdbb3dde4d991b6ae9d2be6e6b74d9a35c6d6bc716fd93844f7a440150a475d970b1a193d9b85a0d7e4e36094128607fb68e5a1c7f295896befe995b3874112ee0d83c75f50450e566f1429ece1df4ef5e738457bb78dcbac1a1f952e8a98c752bcd995d7d499818a499e2d2a71756f2ec07d117391aa7f42dffd890954ff370118ce29af1d81eaf815bd7d99003304ef04edfafa00010011ec04c05a9a1970304d50e28e6ac53ab7d63b0d998d5c0076c7e5210c376ca3ca481b08123eb25541aa45354105da14c6a8d6bde7bb175f9a33297919a8487ecc1563c7d10cb5f857eaf075c7a2b67a065b3d935b2a9b10f79ca7a596c3e145ae2415230055b874b730e6103ddb3cad9a6081421388ef34a8d5b8014bdbbcefd75128172878c375365a0f29071d1898072f2387eaf5bb363c1723e562943214ebb52fe21923a4221bc4fa7b1ff612007398664c520c843dc8f74e3db266858502cfcca0c1bc270f0c965058c3a6b5cefda31c615a18dee529dce593c90179538a0156481d7f5b136aa16e84a423303297badb647729195faa94c1f772aa74a4f5a62a1d9a4100f774c1f8a415d6c55c059a76416b95402de31954df3a3522812975004f74b63d113ba889563b1d31197a89c8520c0507f001c6f2b12c586cc5127406879a13084621317d027a2ccc47ca73e436580847ec04b3dd325623b00681270db6e061abfb72e2e421a0705480e35b4b99a2014ab74f270d4dd09618617cfc901f6a2742aa883b864046c9069a3ca4cfb6552e7f094d5187428a9983deb77bf79719b03a378d086e20b72644b5898af123db605f3096a62f07647d00452b416d74837064d467c9f65c1a0302c1550d710c0e34eba53b80cb34524f8f868a298724122baf64352abc530a0a145000b78b11068f27a34a0ab00ad40b0c20b0c60a257bc58c1ff0a0c919b80b1f02cbb0e563ad178d54ca3ed69547fef51e418c0b3c8027fe80c9887c518eb704a60450ab480fbea44aec8908ede9717039a33b279443f9428c2034960c92b9f94935b655292c450210cd7ee4bba25c990d2aab8da209a376826ab8b2499315f143aec0e5bf866123596ace47473dfc185f2f122744c80ddf0c8ba91bc2d5d916e12a40ee2206f1c2b3efc781aa619888c0a97ee102c0c374914663e4b9050740cc0dc839a619c23ea2208c57536b38690069cd2c52a285f127521680b0276f3c81b94b19868e8b590dac446bd352ee9abf1dca49da1250a7c234eea29fa95b06b7a55bdb6685196115d83581ec3bcb38c65a4eda21b66329366cc13d0881739195ff981df57b01ad326857cc7fa4c813151c518c32a49078c996d431887995f7f802d05a8fb5749d4ecc172e30acc040276896404bb67c5d5b95825c3c4dc6b004b784699805e67886da95ad7311c4860278e2f893aeb53b834b8f8b6bb89055367c885d62fc14d11b39070b06461a7c34b86f88b3595d25c198c9b62b3951bdc01100356233b9532c0707d5484c4ac2baf412310b99360a2a76b1d74363a7af103c058cf56b85166be9b5bca99788dd316c8e4b163277b1ca3b3807b820b0ec1b1e6080515a68780cc3ef9902acf7cd56c59d7bbaafbc5357edfc34b9281f41553cdce864fac7b44f91a9fb9c57a7b7444e6c02b96326c41c72ea18305bcb44edba64d5009d43a27ded903dd614763c86297c1cc71ab4ab3aa772c27bc0bd7a19f3c6a32b579f60b14240a760205767a6b720ef68c7ef77878070040d30907cdb98498837ac5b6ff7e931d7741e914cc83d922125b36ac028c05e7368ec9c8510e682d08322520a1cd7c33f79cc8b7703cf3302246d9d2488be79969ddc34be4020b0461b505278bf4d35023ff85c84d815d041abaa5c2e89820fba36bce5be0b06e5fd128a41e86d7041b7964d968cc347fb9c1c001d00209b0d1f6cc21ce5b6ae1ebd680b4249e7dd04037fd84ab05a3e5c4a6b8530062dfafa000100@@505249202a20485454502f322e300d0a0d0a534d0d0a0d0a00001804000000000000010001000000020000000000040060000000060004000000000408000000000000ef00010001d401250000000180000000ff82418a089d5c0b8170dc79f7df87845887a47e561cc5801f40874148b1275ad1ffb9fe749d3fd4372ed83aa4fe7efbc1fcbefff3f4a7f388e79a82a97a7b0f497f9fbef07f21659fe7e94fe6f4f61e935b4ff3f7de0fe42cb3fcff408b4148b1275ad1ad49e33505023f30408d4148b1275ad1ad5d034ca7b29f07226d61634f53224092b6b9ac1c8558d520a4b6c2ad617b5a54251f01317ad9d07f66a281b0dae053fad0321aa49d13fda992a49685340c8a6adca7e28104416e277fb521aeba0bc8b1e632586d975765c53facd8f7e8cff4a506ea5531149d4ffda97a7b0f49580b2cae05c0b814dc394761986d975765cf53e5497ca589d34d1f43aeba0c41a4c7a98f33a69a3fdf9a68fa1d75d0620d263d4c79a68fbed00177fe8d48e62b03ee697e8d48e62b1e0b1d7f46a4731581d754df5f2c7cfdf6800bbdf43aeba0c41a4c7a9841a6a8b22c5f249c754c5fbef046cfdf6800bbbf408a4148b4a549275906497f83a8f517408a4148b4a549275a93c85f86a87dcd30d25f408a4148b4a549275ad416cf023f31408a4148b4a549275a42a13f8690e4b692d49f50929bd9abfa5242cb40d25fa523b3e94f684c9f518cf73ad7b4fd7b9fefb4005dff4086aec31ec327d785b6007d286f"
	}
	return option, err
}
