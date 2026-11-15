#!/usr/bin/env bash

# 编译本地版本以获取版本号
make
if [ $? -ne 0 ]; then
    echo "make error"
    exit 1
fi

acme_version=$(./bin/acme-manual -v)
echo "build version: $acme_version"

# 交叉编译全平台二进制文件
make -f ./Makefile.cross-compiles
if [ $? -ne 0 ]; then
    echo "make cross-compiles error"
    exit 1
fi

rm -rf ./packages
mkdir -p ./packages

os_all='linux windows'
arch_all='386 amd64 arm arm64'
bins='acme-manual acme-cf acme-aliyun acme-dnspod acme-aws acme-gd acme-dgon acme-gcloud acme-namecheap acme-azure acme-ovh acme-hetznercloud acme-duckdns acme-porkbun acme-namesilo acme-vultr'

for os in $os_all; do
    for arch in $arch_all; do
        pkg_dir_name="acme-nano_${acme_version}_${os}_${arch}"
        pkg_path="./packages/${pkg_dir_name}"

        ext=""
        if [ "x${os}" = x"windows" ]; then
            ext=".exe"
        fi

        missing=0
        for bin in $bins; do
            if [ ! -f "./${bin}_${os}_${arch}${ext}" ]; then
                missing=1
                break
            fi
        done
        if [ $missing -eq 1 ]; then
            continue
        fi

        mkdir -p "${pkg_path}"
        for bin in $bins; do
            mv "./${bin}_${os}_${arch}${ext}" "${pkg_path}/${bin}${ext}"
        done

        cp ./LICENSE ./README.md ./README_zh.md "${pkg_path}/"

        # 打包压缩归档文件
        cd ./packages
        if [ "x${os}" = x"windows" ]; then
            zip -rq "${pkg_dir_name}.zip" "${pkg_dir_name}"
        else
            tar -zcf "${pkg_dir_name}.tar.gz" "${pkg_dir_name}"
        fi
        cd ..
        rm -rf "${pkg_path}"
    done
done

