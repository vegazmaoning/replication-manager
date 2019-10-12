#!/bin/bash
echo "# build replication-manager-osc package"
echo "# Getting branch info"
git status -bs
echo "# Press Return or Space to start if you had build, else Press all other keys to quit and build by yourself"
read -s -n 1 key
if [[ $key != "" ]]; then exit; fi
version=$(git describe --tag)
head=$(git rev-parse --short HEAD)
epoch=$(date +%s)
#old build 
#echo "# Building"
#./build.sh
echo "# Cleaning up previous builds"
rm -rf build
#rm *.tar.gz
rm *.deb
rm *.rpm
mkdir -p build/usr/bin
mkdir -p build/usr/share/replication-manager/dashboard
mkdir -p build/etc/replication-manager
mkdir -p build/etc/systemd/system
mkdir -p build/etc/init.d
mkdir -p build/var/lib/replication-manager
echo "# Copying files to build dir"
cp bin/replication-manager-osc build/usr/bin/
cp bin/replication-manager-cli build/usr/bin/
cp -r etc/* build/etc/replication-manager/
cp -r dashboard/* build/usr/share/replication-manager/dashboard/
cp -r share/* build/usr/share/replication-manager/
cp service/replication-manager.service build/etc/systemd/system/
cp service/replication-manager.init.el6 build/etc/init.d/replication-manager
chmod 755 build/etc/init.d/replication-manager
#cp service/replication-manager-arb.init.el6 build/etc/init.d/replication-manager-arb
#chmod 755 build/etc/init.d/replication-manager-arb
echo "# Building packages"
fpm --epoch $epoch --iteration $head -v $version -C build -s dir -t rpm -n replication-manager-osc .
#fpm --package replication-manager-osc-$version-$head.tar -C build -s dir -t tar -n replication-manager-osc .
#gzip replication-manager-osc-$version-$head.tar.gz
cp service/replication-manager.init.deb7 build/etc/init.d/replication-manager
fpm --epoch $epoch --iteration $head -v $version -C build -s dir -t deb -n replication-manager-osc .
cp *.deb package/
#cp *.tar.gz package/
cp *.rpm /package
echo "genarate package in directory package/"
