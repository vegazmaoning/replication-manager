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
rm *.tar.gz
rm *.deb
rm *.rpm
mkdir -p build/usr/bin
#mkdir -p build/usr/share/replication-manager/dashboard
mkdir -p build/etc/replication-manager
mkdir -p build/etc/systemd/system
mkdir -p build/etc/init.d
mkdir -p build/var/lib/replication-manager
echo "# Copying files to build dir"
cp bin/replication-manager-arb build/usr/bin/
cp bin/replication-manager-cli build/usr/bin/
cp service/replication-manager-arb.service build/etc/systemd/system/
cp service/replication-manager-arb.init.el6 build/etc/init.d/replication-manager-arb
chmod 755 build/etc/init.d/replication-manager-arb
echo "# Building packages"
fpm --epoch $epoch --iteration $head -v $version -C build -s dir -t rpm -n replication-manager-arb .
#fpm --package replication-manager-$version-$head.tar -C build -s dir -t tar -n replication-manager-arb .
#gzip replication-manager-arb-$version-$head.tar.gz
cp service/replication-manager-arb.init.deb7 build/etc/init.d/replication-manager-arb
fpm --epoch $epoch --iteration $head -v $version -C build -s dir -t deb -n replication-manager-arb .
mv *.deb package/
#mv *.tar.gz package/
mv *.rpm /package
echo "genarate package in directory package/"
