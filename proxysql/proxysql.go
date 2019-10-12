package proxysql

import (
	"fmt"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type ProxySQL struct {
	Connection *sqlx.DB
	User       string
	Password   string
	Port       string
	Host       string
	WriterHG   string
	ReaderHG   string
}

func (psql *ProxySQL) Connect() error {
	ProxysqlConfig := mysql.Config{
		User:        psql.User,
		Passwd:      psql.Password,
		Net:         "tcp",
		Addr:        fmt.Sprintf("%s:%s", psql.Host, psql.Port),
		Timeout:     time.Second * 5,
		ReadTimeout: time.Second * 15,
	}

	var err error
	psql.Connection, err = sqlx.Connect("mysql", ProxysqlConfig.FormatDSN())
	if err != nil {
		return fmt.Errorf("Could not connect to ProxySQL (%s)", err)
	}
	return nil
}

func (psql *ProxySQL) AddServer(host string, port string) error {
	log.Printf("insert mysql_servers %s:%s", host, port)
	sql := fmt.Sprintf("INSERT INTO mysql_servers (hostname, port) VALUES('%s','%s')", host, port)
	_, err := psql.Connection.Exec(sql)
	return err
}

func (psql *ProxySQL) AddOfflineServer(host string, port string) error {
	log.Printf("insert mysql_servers %s:%s to hostgroup 666", host, port)
	sql := fmt.Sprintf("INSERT INTO mysql_servers (hostgroup_id, hostname, port) VALUES('666', '%s','%s')", host, port)
	_, err := psql.Connection.Exec(sql)
	return err
}

func (psql *ProxySQL) SetOffline(host string, port string) error {
	log.Printf("update mysql_servers %s:%s to hostgroup 666", host, port)
	sql := fmt.Sprintf("UPDATE mysql_servers SET hostgroup_id='666' WHERE hostname='%s' AND port='%s'", host, port)
	_, err := psql.Connection.Exec(sql)
	return err
}

func (psql *ProxySQL) SetOfflineSoft(host string, port string) error {
	log.Printf("update mysql_servers %s:%s to status=OFFLINE_SOFT, hostgrout_id=%s", host, port, psql.ReaderHG)
	sql := fmt.Sprintf("UPDATE mysql_servers SET status='OFFLINE_SOFT', hostgroup_id='%s' WHERE hostname='%s' AND port='%s'", psql.ReaderHG, host, port)
	_, err := psql.Connection.Exec(sql)
	return err
}

func (psql *ProxySQL) SetOnline(host string, port string) error {
	log.Printf("update mysql_servers %s:%s to status=ONLINE", host, port)
	sql := fmt.Sprintf("UPDATE mysql_servers SET status='ONLINE' WHERE hostname='%s' AND port='%s'", host, port)
	_, err := psql.Connection.Exec(sql)
	return err
}

func (psql *ProxySQL) SetWriter(host string, port string) error {
	sql := fmt.Sprintf("UPDATE mysql_servers SET status='ONLINE', hostgroup_id='%s' WHERE hostname='%s' AND port='%s'", psql.WriterHG, host, port)
	_, err := psql.Connection.Exec(sql)
	return err
}

func (psql *ProxySQL) SetReader(host string, port string) error {
	sql := fmt.Sprintf("UPDATE mysql_servers SET status='ONLINE', hostgroup_id='%s' WHERE hostname='%s' AND port='%s'", psql.ReaderHG, host, port)
	_, err := psql.Connection.Exec(sql)
	return err
}

func (psql *ProxySQL) LoadServersToRuntime() error {
	_, err := psql.Connection.Exec("LOAD MYSQL SERVERS TO RUNTIME")
	return err
}

func (psql *ProxySQL) GetStatsForHostRead(host string, port string) (string, string, int, int, int, int, error) {
	var (
		hostgroup string
		status    string
		connused  int
		byteout   int
		bytein    int
		latency   int
	)
	sql := fmt.Sprintf("SELECT hostgroup, status, ConnUsed, Bytes_data_sent , Bytes_data_recv , Latency_us FROM stats.stats_mysql_connection_pool INNER JOIN mysql_replication_hostgroups ON mysql_replication_hostgroups.reader_hostgroup=hostgroup  WHERE srv_host='%s' AND srv_port='%s'", host, port)
	row := psql.Connection.QueryRow(sql)
	err := row.Scan(&hostgroup, &status, &connused, &byteout, &bytein, &latency)
	return hostgroup, status, connused, byteout, bytein, latency, err
}

func (psql *ProxySQL) GetStatsForHostWrite(host string, port string) (string, string, int, int, int, int, error) {
	var (
		hostgroup string
		status    string
		connused  int
		byteout   int
		bytein    int
		latency   int
	)
	sql := fmt.Sprintf("SELECT hostgroup, status, ConnUsed, Bytes_data_sent , Bytes_data_recv , Latency_us FROM stats.stats_mysql_connection_pool INNER JOIN mysql_replication_hostgroups ON mysql_replication_hostgroups.writer_hostgroup=hostgroup  WHERE srv_host='%s' AND srv_port='%s'", host, port)
	row := psql.Connection.QueryRow(sql)
	err := row.Scan(&hostgroup, &status, &connused, &byteout, &bytein, &latency)
	return hostgroup, status, connused, byteout, bytein, latency, err
}

func (psql *ProxySQL) GetVersion() string {
	var version string
	sql := "SELECT @@admin-version"
	row := psql.Connection.QueryRow(sql)
	row.Scan(&version)
	return version
}

func (psql *ProxySQL) GetHostsRuntime() (string, error) {
	var h string
	err := psql.Connection.Get(&h, "SELECT GROUP_CONCAT(host) AS hostlist FROM (SELECT hostname || ':' || port AS host FROM runtime_mysql_servers)")
	return h, err
}

func (psql *ProxySQL) Truncate() error {
	_, err := psql.Connection.Exec("DELETE FROM mysql_servers")
	return err
}

func (psql *ProxySQL) AddUser(User string, Password string) error {
	_, err := psql.Connection.Exec("REPLACE INTO mysql_users(username,password) VALUES('" + User + "','" + Password + "')")
	if err != nil {
		return err
	}
	_, err = psql.Connection.Exec("LOAD MYSQL USERS TO RUNTIME")
	return err
}
