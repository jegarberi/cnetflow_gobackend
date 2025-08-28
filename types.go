package main

type Exporter struct {
	ID      uint64 `json:"id" db:"id"`
	IP_Bin  int64  `json:"ip_bin" db:"ip_bin"`
	IP_Inet string `json:"ip_inet" db:"ip_inet"`
	Name    string `json:"name" db:"name"`
}

type Interface struct {
	ID       uint64 `json:"id" db:"id"`
	Exporter string `json:"exporter" db:"exporter"`
	Snmp_if  uint64 `json:"snmp_if" db:"snmp_if"`
	Descr    string `json:"description" db:"description"`
	Alias    string `json:"alias" db:"alias"`
	Speed    int64  `json:"speed" db:"speed"`
	Enabled  bool   `json:"enabled" db:"enabled"`
}
