package setting

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// BEpusdt (https://github.com/v03413/BEpusdt) is a self-hosted USDT gateway.
// The site creates orders through its native create-transaction API and shows
// the receiving address in the wallet page instead of redirecting.
var BEpusdtAddress = ""
var BEpusdtToken = ""

// BEpusdtTimeout is the order lifetime in seconds passed to BEpusdt.
var BEpusdtTimeout = 1800

// BEpusdtPayMethods lists the networks shown on the wallet page. Each type is
// a BEpusdt trade_type such as usdt.trc20; min_topup uses top-up amount units.
var BEpusdtPayMethods = `[
  {"name":"USDT-TRC20","type":"usdt.trc20","icon":"SiTether","min_topup":"10"},
  {"name":"USDT-BEP20","type":"usdt.bep20","icon":"SiTether","min_topup":"10"},
  {"name":"USDT-Polygon","type":"usdt.polygon","icon":"SiTether","min_topup":"10"},
  {"name":"USDT-Arbitrum","type":"usdt.arbitrum","icon":"SiTether","min_topup":"10"},
  {"name":"USDT-X Layer","type":"usdt.xlayer","icon":"SiTether","min_topup":"10"},
  {"name":"USDT-Plasma","type":"usdt.plasma","icon":"SiTether","min_topup":"10"},
  {"name":"USDT-Solana","type":"usdt.solana","icon":"SiTether","min_topup":"10"},
  {"name":"USDT-ERC20","type":"usdt.erc20","icon":"SiTether","min_topup":"50"}
]`

// ValidateBEpusdtPayMethods rejects a value that is not a JSON array of
// methods, each with a name and a type.
func ValidateBEpusdtPayMethods(value string) error {
	var methods []map[string]string
	if err := common.UnmarshalJsonStr(value, &methods); err != nil {
		return common.NewMessage("BEpusdt payment methods must be a JSON array")
	}
	for _, method := range methods {
		if strings.TrimSpace(method["type"]) == "" || strings.TrimSpace(method["name"]) == "" {
			return common.NewMessage("Each BEpusdt payment method needs a name and a type")
		}
	}
	return nil
}

// GetBEpusdtPayMethods parses BEpusdtPayMethods and drops entries without a
// type or name. An invalid JSON value yields no methods.
func GetBEpusdtPayMethods() []map[string]string {
	var methods []map[string]string
	if err := common.UnmarshalJsonStr(BEpusdtPayMethods, &methods); err != nil {
		return nil
	}
	valid := make([]map[string]string, 0, len(methods))
	for _, method := range methods {
		if strings.TrimSpace(method["type"]) == "" || strings.TrimSpace(method["name"]) == "" {
			continue
		}
		valid = append(valid, method)
	}
	return valid
}
