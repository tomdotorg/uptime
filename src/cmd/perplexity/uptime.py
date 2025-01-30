import socket
import time


def read_config_file(file_path):
	with open(file_path, 'r') as file:
		lines = file.readlines()
		hosts = [line.strip() for line in lines if not line.startswith('#')]
	return hosts


def check_host_status(host, port):
	try:
		with socket.create_connection((host, port), timeout=5) as s:
			return True
	except (socket.timeout, ConnectionRefusedError, OSError):
		return False


def main():
	hosts, status = read_config()
	while True:
		for host in hosts:
			host, port = host.split(':')
			port = int(port)
			new_status = 'up' if check_host_status(host, port) else 'down'
			if new_status != status[host+":"+str(port)]:
				print(f'Host {host} is {new_status}')
				status[host] = new_status
		time.sleep(5)


def read_config():
	config_file = 'config.txt'  # Replace with your actual file path
	hosts = read_config_file(config_file)
	status = {host: 'down' for host in hosts}
	return hosts, status


if __name__ == '__main__':
	main()
