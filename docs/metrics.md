# Metrics

The VPN Seed Server runs an in-process [OpenVPN exporter](https://github.com/kumina/openvpn_exporter) exposing Prometheus style metrics on <http://0.0.0.0:15000/metrics>

## Server statistics

For server status files (both version 2 and 3), the exporter generates metrics that may look like this:

```
# HELP openvpn_netstat_Tcp_OutSegs Statistic TcpOutSegs.
# TYPE openvpn_netstat_Tcp_OutSegs untyped
openvpn_netstat_Tcp_OutSegs 6.3061057e+07
# HELP openvpn_netstat_Tcp_RetransSegs Statistic TcpRetransSegs.
# TYPE openvpn_netstat_Tcp_RetransSegs untyped
openvpn_netstat_Tcp_RetransSegs 34409
# HELP openvpn_server_client_received_bytes_total Amount of data received over a connection on the VPN server, in bytes.
# TYPE openvpn_server_client_received_bytes_total counter
openvpn_server_client_received_bytes_total{common_name="vpn-seed-client",connection_time="1780486694",real_address="10.112.238.119:44618",status_path="/srv/status/openvpn.status",username="UNDEF",virtual_address=""} 7.681531198e+09
openvpn_server_client_received_bytes_total{common_name="vpn-seed-client",connection_time="1780486694",real_address="10.122.16.171:36284",status_path="/srv/status/openvpn.status",username="UNDEF",virtual_address=""} 2.453020086e+09
openvpn_server_client_received_bytes_total{common_name="vpn-seed-client",connection_time="1780486695",real_address="10.105.167.172:40288",status_path="/srv/status/openvpn.status",username="UNDEF",virtual_address=""} 4.296638345e+09
openvpn_server_client_received_bytes_total{common_name="vpn-shoot-client-0",connection_time="1780632117",real_address="10.105.167.154:55366",status_path="/srv/status/openvpn.status",username="UNDEF",virtual_address=""} 6.646300848e+09
openvpn_server_client_received_bytes_total{common_name="vpn-shoot-client-1",connection_time="1780891529",real_address="10.111.94.31:33594",status_path="/srv/status/openvpn.status",username="UNDEF",virtual_address=""} 5.519587e+06
# HELP openvpn_server_client_sent_bytes_total Amount of data sent over a connection on the VPN server, in bytes.
# TYPE openvpn_server_client_sent_bytes_total counter
openvpn_server_client_sent_bytes_total{common_name="vpn-seed-client",connection_time="1780486694",real_address="10.112.238.119:44618",status_path="/srv/status/openvpn.status",username="UNDEF",virtual_address=""} 7.790259577e+09
openvpn_server_client_sent_bytes_total{common_name="vpn-seed-client",connection_time="1780486694",real_address="10.122.16.171:36284",status_path="/srv/status/openvpn.status",username="UNDEF",virtual_address=""} 1.726189903e+09
openvpn_server_client_sent_bytes_total{common_name="vpn-seed-client",connection_time="1780486695",real_address="10.105.167.172:40288",status_path="/srv/status/openvpn.status",username="UNDEF",virtual_address=""} 4.106725334e+09
openvpn_server_client_sent_bytes_total{common_name="vpn-shoot-client-0",connection_time="1780632117",real_address="10.105.167.154:55366",status_path="/srv/status/openvpn.status",username="UNDEF",virtual_address=""} 7.078589822e+09
openvpn_server_client_sent_bytes_total{common_name="vpn-shoot-client-1",connection_time="1780891529",real_address="10.111.94.31:33594",status_path="/srv/status/openvpn.status",username="UNDEF",virtual_address=""} 1.01671e+07
# HELP openvpn_server_connected_clients Number Of Connected Clients
# TYPE openvpn_server_connected_clients gauge
openvpn_server_connected_clients{status_path="/srv/status/openvpn.status"} 5
# HELP openvpn_server_route_last_reference_time_seconds Time at which a route was last referenced, in seconds.
# TYPE openvpn_server_route_last_reference_time_seconds gauge
openvpn_server_route_last_reference_time_seconds{common_name="vpn-seed-client",real_address="10.105.167.172:40288",status_path="/srv/status/openvpn.status",virtual_address="da:e6:7a:6f:80:a4@0"} 1.78092127e+09
openvpn_server_route_last_reference_time_seconds{common_name="vpn-seed-client",real_address="10.112.238.119:44618",status_path="/srv/status/openvpn.status",virtual_address="02:8a:d4:6d:0d:26@0"} 1.78092127e+09
openvpn_server_route_last_reference_time_seconds{common_name="vpn-seed-client",real_address="10.122.16.171:36284",status_path="/srv/status/openvpn.status",virtual_address="fa:e8:2e:f9:a3:69@0"} 1.78092127e+09
openvpn_server_route_last_reference_time_seconds{common_name="vpn-shoot-client-0",real_address="10.105.167.154:55366",status_path="/srv/status/openvpn.status",virtual_address="22:0b:a9:6a:f4:57@0"} 1.78092127e+09
openvpn_server_route_last_reference_time_seconds{common_name="vpn-shoot-client-1",real_address="10.111.94.31:33594",status_path="/srv/status/openvpn.status",virtual_address="12:61:89:f5:07:c5@0"} 1.78092127e+09
# HELP openvpn_status_update_time_seconds UNIX timestamp at which the OpenVPN statistics were updated.
# TYPE openvpn_status_update_time_seconds gauge
openvpn_status_update_time_seconds{status_path="/srv/status/openvpn.status"} 1.780921271e+09
# HELP openvpn_up Whether scraping OpenVPN's metrics was successful.
# TYPE openvpn_up gauge
openvpn_up{status_path="/srv/status/openvpn.status"} 1
```
