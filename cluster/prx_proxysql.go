package cluster

import (
	"fmt"
	"strconv"

	"github.com/signal18/replication-manager/dbhelper"
	"github.com/signal18/replication-manager/proxysql"
	"github.com/signal18/replication-manager/state"
)

func connectProxysql(proxy *Proxy) (proxysql.ProxySQL, error) {
	psql := proxysql.ProxySQL{
		User:     proxy.User,
		Password: proxy.Pass,
		Host:     proxy.Host,
		Port:     proxy.Port,
		WriterHG: fmt.Sprintf("%d", proxy.WriterHostgroup),
		ReaderHG: fmt.Sprintf("%d", proxy.ReaderHostgroup),
	}

	var err error
	err = psql.Connect()
	if err != nil {
		return psql, err
	}
	return psql, nil
}

func (cluster *Cluster) initProxysql(proxy *Proxy) {
	if cluster.conf.ProxysqlOn == false {
		return
	}

	psql, err := connectProxysql(proxy)
	if err != nil {
		cluster.sme.AddState("ERR00051", state.State{ErrType: "ERROR", ErrDesc: fmt.Sprintf(clusterError["ERR00051"], err), ErrFrom: "MON"})
		return
	}
	defer psql.Connection.Close()

	for _, s := range cluster.servers {
		if cluster.conf.ProxysqlBootstrap {
			err = psql.AddServer(s.SMNetworkPair[s.Host], s.Port)
			if err != nil {
				cluster.LogPrintf(LvlErr, "ProxySQL could not add server %s (%s)", s.URL, err)
			}
		}
		if s.State == stateUnconn {
			err = psql.AddOfflineServer(s.SMNetworkPair[s.Host], s.Port)
			if err != nil {
				cluster.LogPrintf(LvlErr, "ProxySQL could not add server %s as offline (%s)", s.URL, err)
			}
		}
	}
	err = psql.LoadServersToRuntime()
	if err != nil {
		cluster.LogPrintf(LvlErr, "ProxySQL could not load servers to runtime (%s)", err)
	}
}

func (cluster *Cluster) failoverProxysql(proxy *Proxy) {
	psql, err := connectProxysql(proxy)
	if err != nil {
		cluster.sme.AddState("ERR00051", state.State{ErrType: "ERROR", ErrDesc: fmt.Sprintf(clusterError["ERR00051"], err), ErrFrom: "MON"})
		return
	}
	if err != nil {
		cluster.sme.AddState("ERR00051", state.State{ErrType: "ERROR", ErrDesc: fmt.Sprintf(clusterError["ERR00051"], err), ErrFrom: "MON"})
		return
	}
	defer psql.Connection.Close()
	for _, s := range cluster.servers {
		if s.State == stateUnconn {
			err = psql.SetOffline(s.SMNetworkPair[s.Host], s.Port)
			if err != nil {
				cluster.LogPrintf(LvlErr, "ProxySQL could not set server %s offline (%s)", s.URL, err)
			}
		}
	}
	err = psql.LoadServersToRuntime()
	if err != nil {
		cluster.LogPrintf(LvlErr, "ProxySQL could not load servers to runtime (%s)", err)
	}
}

func (cluster *Cluster) refreshProxysql(proxy *Proxy) {
	if cluster.conf.ProxysqlOn == false {
		return
	}

	psql, err := connectProxysql(proxy)
	if err != nil {
		cluster.sme.AddState("ERR00051", state.State{ErrType: "ERROR", ErrDesc: fmt.Sprintf(clusterError["ERR00051"], err), ErrFrom: "MON"})
		return
	}
	defer psql.Connection.Close()
	proxy.Version = psql.GetVersion()

	var updated bool
	proxy.BackendsWrite = nil
	proxy.BackendsRead = nil

	for _, s := range cluster.servers {
		proxysqlHostgroup, proxysqlServerStatus, proxysqlServerConnections, proxysqlByteOut, proxysqlByteIn, proxysqlLatency, err := psql.GetStatsForHostWrite(s.SMNetworkPair[s.Host], s.Port)
		var bke = Backend{
			Host:           s.Host,
			Port:           s.Port,
			Status:         s.State,
			PrxName:        s.URL,
			PrxStatus:      proxysqlServerStatus,
			PrxConnections: strconv.Itoa(proxysqlServerConnections),
			PrxByteIn:      strconv.Itoa(proxysqlByteOut),
			PrxByteOut:     strconv.Itoa(proxysqlByteIn),
			PrxLatency:     strconv.Itoa(proxysqlLatency),
			PrxHostgroup:   proxysqlHostgroup,
		}

		s.MxsServerName = s.URL
		s.ProxysqlHostgroup = proxysqlHostgroup
		s.MxsServerStatus = proxysqlServerStatus

		if err != nil {

			s.MxsServerStatus = "REMOVED"
			bke.PrxStatus = "REMOVED"
		} else {
			proxy.BackendsWrite = append(proxy.BackendsWrite, bke)
		}
		rproxysqlHostgroup, rproxysqlServerStatus, rproxysqlServerConnections, rproxysqlByteOut, rproxysqlByteIn, rproxysqlLatency, err := psql.GetStatsForHostRead(s.SMNetworkPair[s.Host], s.Port)
		var bkeread = Backend{
			Host:           s.Host,
			Port:           s.Port,
			Status:         s.State,
			PrxName:        s.URL,
			PrxStatus:      rproxysqlServerStatus,
			PrxConnections: strconv.Itoa(rproxysqlServerConnections),
			PrxByteIn:      strconv.Itoa(rproxysqlByteOut),
			PrxByteOut:     strconv.Itoa(rproxysqlByteIn),
			PrxLatency:     strconv.Itoa(rproxysqlLatency),
			PrxHostgroup:   rproxysqlHostgroup,
		}
		if err == nil {
			proxy.BackendsRead = append(proxy.BackendsRead, bkeread)
		}
		// if ProxySQL and replication-manager states differ, resolve the conflict
		if bke.PrxStatus == "OFFLINE_HARD" && s.State == stateSlave {
			cluster.LogPrintf(LvlDbg, "ProxySQL setting online rejoining server %s", s.URL)
			err = psql.SetReader(s.SMNetworkPair[s.Host], s.Port)
			if err != nil {
				cluster.LogPrintf(LvlErr, "ProxySQL could not set %s as reader (%s)", s.URL, err)
			}
			updated = true
		}

		// if server is Standalone, set offline in ProxySQL
		if s.State == stateUnconn && bke.PrxStatus == "ONLINE" {
			cluster.LogPrintf(LvlDbg, "ProxySQL setting offline standalone server %s", s.URL)
			err = psql.SetOffline(s.SMNetworkPair[s.Host], s.Port)
			if err != nil {
				cluster.LogPrintf(LvlErr, "ProxySQL could not set %s as offline (%s)", s.URL, err)
			}
			updated = true

			// if the server comes back from a previously failed or standalone state, reintroduce it in
			// the appropriate HostGroup
		} else if s.PrevState == stateUnconn || s.PrevState == stateFailed {
			if s.State == stateMaster {
				err = psql.SetWriter(s.SMNetworkPair[s.Host], s.Port)
				if err != nil {
					cluster.LogPrintf(LvlErr, "ProxySQL could not set %s as writer (%s)", s.URL, err)
				}
				updated = true
			} else if s.IsSlave {
				err = psql.SetReader(s.SMNetworkPair[s.Host], s.Port)
				if err != nil {
					cluster.LogPrintf(LvlErr, "ProxySQL could not set %s as reader (%s)", s.URL, err)
				}
				updated = true
			}
		}
		// load the grants
		if s.IsMaster() && cluster.conf.ProxysqlCopyGrants {
			myprxusermap, err := dbhelper.GetProxySQLUsers(psql.Connection)
			if err != nil {
				cluster.sme.AddState("ERR00053", state.State{ErrType: "WARNING", ErrDesc: fmt.Sprintf(clusterError["ERR00053"], err), ErrFrom: "MON"})
			}
			uniUsers := make(map[string]dbhelper.Grant)
			dupUsers := make(map[string]string)

			for _, u := range s.Users {
				user, ok := uniUsers[u.User+":"+u.Password]
				if ok {
					dupUsers[user.User] = user.User
					cluster.sme.AddState("ERR00057", state.State{ErrType: "WARNING", ErrDesc: fmt.Sprintf(clusterError["ERR00057"], user.User), ErrFrom: "MON"})
				} else {
					if u.Password != "" {
						uniUsers[u.User+":"+u.Password] = u
					}
				}
			}

			for _, user := range uniUsers {
				if _, ok := myprxusermap[user.User+":"+user.Password]; !ok {
					cluster.LogPrintf(LvlInfo, "Add ProxySQL user %s ", user.User)
					err := psql.AddUser(user.User, user.Password)
					if err != nil {
						cluster.sme.AddState("ERR00054", state.State{ErrType: "WARNING", ErrDesc: fmt.Sprintf(clusterError["ERR00054"], err), ErrFrom: "MON"})

					}
				}
			}
		}
	}
	if updated {
		err = psql.LoadServersToRuntime()
		if err != nil {
			cluster.LogPrintf(LvlErr, "ProxySQL could not load servers to runtime (%s)", err)
		}
	}
}

func (cluster *Cluster) setMaintenanceProxysql(proxy *Proxy, s *ServerMonitor) {
	if cluster.conf.ProxysqlOn == false {
		return
	}

	psql, err := connectProxysql(proxy)
	if err != nil {
		cluster.sme.AddState("ERR00051", state.State{ErrType: "ERROR", ErrDesc: fmt.Sprintf(clusterError["ERR00051"], err), ErrFrom: "MON"})
		return
	}
	defer psql.Connection.Close()

	if s.IsMaintenance {
		err = psql.SetOfflineSoft(s.SMNetworkPair[s.Host], s.Port)
		if err != nil {
			cluster.LogPrintf(LvlErr, "ProxySQL could not set %s:%s as offline_soft (%s)", s.Host, s.Port, err)
		}
	} else {
		err = psql.SetOnline(s.Host, s.Port)
		if err != nil {
			cluster.LogPrintf(LvlErr, "ProxySQL could not set %s:%s as online (%s)", s.Host, s.Port, err)
		}
	}
	err = psql.LoadServersToRuntime()
	if err != nil {
		cluster.LogPrintf(LvlErr, "ProxySQL could not load servers to runtime (%s)", err)
	}
}
