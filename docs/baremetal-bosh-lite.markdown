# How bosh-lite was installed

```
# silence is golden
touch ~/.hushlogin

# prereqs
apt-get install git vim unzip wget

#
# Vagrant
# Download latest debian package from https://www.vagrantup.com/downloads.html
dpkg -i vagrant_xyz_x86_64.deb

#
# VirtualBox
#

# register the package source
echo 'deb http://download.virtualbox.org/virtualbox/debian vivid contrib' >> /etc/apt/sources.list

# trust the key
wget -q https://www.virtualbox.org/download/oracle_vbox.asc -O- | sudo apt-key add -

# install
apt-get update
apt-get install virtualbox-5.x

#
# bosh-lite
#

# get the latest stemcell
wget --content-disposition https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent

# get the latest source
mkdir -p workspace
git clone https://github.com/cloudfoundry/bosh-lite
cd bosh-lite

# start the VM
vagrant up

# Make sure bin/add-route actually routes to the correct IP, e.g. for acceptance it's 192.168.150.4
bin/add-route

#
# Ruby
#
apt-get install software-properties-common
apt-add-repository ppa:brightbox/ruby-ng
apt-get update
apt-get install ruby2.3

# from now on, no more rdoc nor ri
echo 'gem: --no-rdoc --no-ri' >> ~/.gemrc
gem install bundler bosh_cli

#
# spruce
#
wget https://github.com/geofffranks/spruce/releases/download/x.y.z/spruce-linux-amd64
chmod +x spruce-linux-amd64
mv spruce-linux-amd64 /usr/local/bin/spruce
```

# Expose port-forwards to external connections

Add `GatewayPorts yes` to `/etc/ssh/ssh_config` on the baremetals.

# Update bosh-lite

In order to update bosh-lite or re-create the vagrant vm do:

```
cd workspace/bosh-lite
vagrant destroy
git pull
vagrant box update
vim Vagrantfile
```

In the Vagrantfile increase the number of CPUs and add port forwarding:

```ruby
Vagrant.configure('2') do |config|
  config.vm.box = 'cloudfoundry/bosh-lite'
  
  config.vm.provider :virtualbox do |v, override|
    override.vm.box_version = '9000.94.0' # ci:replace

    # ADD THE FOLLOWING 4 LINES:
    v.cpus = 7
    override.vm.network "forwarded_port", guest: 25555, host: 25555
    override.vm.network "forwarded_port", guest: 80, host: 80
    override.vm.network "forwarded_port", guest: 443, host: 443

    # To use a different IP address for the bosh-lite director, uncomment this line:
    # override.vm.network :private_network, ip: '192.168.59.4', id: :local
  end
  ...
```

For bosh2, also change `bin/add-route` to contain `gw="192.168.100.4"`.

Start bosh-lite and create our users:

```
vagrant up
bosh create user admin <password>
```

Where `password` comes from LastPass `ci-config` notes `bosh-password` or `bosh2-password`.
# Install cf

```
wget -q -O - https://packages.cloudfoundry.org/debian/cli.cloudfoundry.org.key | sudo apt-key add -
echo "deb http://packages.cloudfoundry.org/debian stable main" | sudo tee /etc/apt/sources.list.d/cloudfoundry-cli.list
sudo apt-get update
sudo apt-get install cf-cli
```

# ssh key setup for Concourse
The following might be necessary in the future, when we use bosh-lites that are not set up through vagrant and hence need other ways to port-foward to the VirtualBox VM. Note that this does not work entirely yet. While the tunnels can be set up successfully, using them with `curl` or `cf` does not work. It might be necessary to make more system changes on the baremetal machine to expose the ports to external connections.

On the baremetal machine, create an executable file `/root/workspace/set-up-authorized-keys.sh`:

```bash
#!/bin/bash -ex

export SSH_PUB_KEY=$(ssh-keygen -y -f ~/.ssh/id_rsa_boshlite)

cd ~/workspace/bosh-lite
vagrant ssh -c "sudo sh -c "'"'"mkdir -p /root/.ssh && echo $SSH_PUB_KEY > /root/.ssh/authorized_keys"'"'""
```

In the Concourse task script, do:

```bash
function setup_ssh() {
  echo "$SSH_KEY" >$PWD/.ssh-key
  chmod 600 $PWD/.ssh-key
  mkdir -p ~/.ssh && chmod 700 ~/.ssh
  local ip=$(echo $SSH_CONNECTION_STRING | cut -d "@" -f2)
  ssh-keyscan -t rsa,dsa $ip >>~/.ssh/known_hosts
}

function is_tunnel_up() {
  ssh $conn_str "nc -z localhost $1"
}

function wait_for_tunnel() {
  local result=1
  local n=0
  until [ $n -ge 5 ]; do
    if is_tunnel_up $1; then
      result=0
      break
    fi

    n=$(($n + 1))
    sleep 1
  done

  return $result
}

setup_ssh
conn_str="$SSH_CONNECTION_STRING -i $PWD/.ssh-key"
ssh $conn_str "~/workspace/set-up-authorized-keys.sh "'"'"$SSH_KEY"'"'""
ssh $conn_str "echo "'"'"$SSH_KEY"'"'" > ~/.ssh/id_rsa_boshlite"
ssh $conn_str "chmod 600 ~/.ssh/id_rsa_boshlite"
ssh $conn_str "ssh root@192.168.50.4 -i ~/.ssh/id_rsa_boshlite -L 80:10.244.0.34:80 -N" &
ssh $conn_str "ssh root@192.168.50.4 -i ~/.ssh/id_rsa_boshlite -L 443:10.244.0.34:443 -N" &
wait_for_tunnel 80
wait_for_tunnel 443
ssh $conn_str "rm -f ~/.ssh/id_rsa_boshlite"

```

# Probably deprecated


## Set up IP routing for bosh-lites

### bosh1

ssh into the bare-metal box 'bosh1' and execute:

```
echo 1 | sudo tee /proc/sys/net/ipv4/ip_forward
ip route add 10.250.0.0/16 via 192.168.50.4

cd ~/workspace/bosh-lite
vagrant ssh
sudo ip route add 10.155.171.0/24 via 192.168.50.1 dev eth1
sudo ip route add 10.155.248.0/24 via 192.168.50.1 dev eth1
```

### bosh2

ssh into the bare-metal box 'bosh2' and execute:

```
echo 1 | sudo tee /proc/sys/net/ipv4/ip_forward
ip route add 10.250.0.0/16 via 192.168.100.4

cd ~/workspace/bosh-lite
vagrant ssh
sudo ip route add 10.155.171.0/24 via 192.168.100.1 dev eth1
sudo ip route add 10.155.248.0/24 via 192.168.100.1 dev eth1
```

# Set up IP routing for bosh-lites

## acceptance

ssh into the bare-metal box 'acceptance' and execute:

```
echo 1 | sudo tee /proc/sys/net/ipv4/ip_forward
ip route add 10.250.0.0/16 via 192.168.150.4

cd ~/workspace/bosh-lite
vagrant ssh
sudo ip route add 10.155.248.0/24 via 192.168.150.1 dev eth1
```
