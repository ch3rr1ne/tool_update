#!/bin/bash
yellow(){
    echo -e "\033[33m\033[01m$1\033[0m"
}
script_dir=$(dirname "$0")
download_dir="$script_dir/tool"
version_file="$script_dir/$download_dir/version.txt"
#检测是否第一次运行（检测存放版本号文件是否存在）
if [ ! -f "$version_file" ]; then
mkdir -p "$download_dir"
mkdir "$download_dir/history"
touch "$version_file"
usr=$(whoami)
echo "0 */12 * * * $(pwd)/tool.sh" >> /var/spool/cron/crontabs/$usr
apt install wget -y
fi
#是否存在版本号记录和下载功能
function check_version(){
saved_version=$(grep -E "^$name" "$version_file" | cut -d' ' -f3-)
if [ "$saved_version" == "$get_version" ]; then
        yellow "$name 已是最新版本: $saved_version"
    else
        yellow "$name 当前版本: $saved_version , 最新版本: $get_version"
            mv "$download_dir/$savename" "$download_dir/history/"
            ls -t "$download_dir/history/${name}"* | tail -n +6 | xargs rm -f
            start_download
#更新版本号
    if ! grep -q "^$name" "$version_file"; then
        echo "$name = $get_version" >> "$version_file"
    else
        sed -i -E "s|^$name.*$|$name = $get_version|" "$version_file"
    fi
fi
}
###下载相关###
function start_download(){
download_path="$download_dir/$savename"
if wget --spider "$dl_url" 2>&1 | grep -q "Accept-Ranges: bytes"; then
    resume_flag="-c"
else
    resume_flag=""
fi
retries=0
    until wget $resume_flag --timeout=30 --tries=3 -O "$download_path" "$dl_url"; do
        retries=$((retries+1))
        if [ $retries -ge 10 ]; then
            yellow "下载失败，网络超时"
        fi
            sleep 1
    done
}
###下载小飞机###
name="MSIAfterburner"
    echo "开始下载 $name"
get_version=$(curl -s https://www.guru3d.com/download/msi-afterburner-beta-download/ | grep -oP '<title>.*</title>' | awk -F'<title>|</title>' '{print $2}' | awk -F'r ' '{print $2}' | sed 's/ Download//')
DL=$(echo $get_version | tr -d ' .')
dl_url="https://ftp.nluug.nl/pub/games/PC/guru3d/afterburner/[Guru3D]-MSIAfterburnerSetup"$DL".zip"
savename=$(echo "$dl_url" | grep -oP '(?<=-).*')
check_version
###下载CPU-Z###
name="CPU-Z"
    echo "开始下载 $name"
get_version=$(curl -s https://www.cpuid.com/softwares/cpu-z.html | grep -oP 'Version \K\d+\.\d+' | sort -V | tail -n 1)
dl_url=https://download.cpuid.com/cpu-z/cpu-z_"$get_version"-cn.exe
savename=$(basename "$dl_url")
check_version
###下载GPU-Z###
name="GPU-Z"
    echo "开始下载GPU-Z"
dl_url="https://ftp.nluug.nl/pub/games/PC/guru3d/generic/GPU-Z-[Guru3D.com].zip"
get_version=$(curl -s https://www.guru3d.com/download/gpu-z-download-techpowerup/ | grep -oP '<title>.*</title>' | awk -F'<title>|</title>' '{print $2}' | awk -F'v' '{print $2}' | awk '{print $1}')
filename=$(basename $dl_url)
savename="${filename/\[Guru3D.com\]/$get_version}"
check_version
###下载HWINFO###
name="HWINFO"
    echo "开始下载HWINFO"
get_version=$(curl -s https://www.hwinfo.com/download/ | grep -oP '<sub>Version \K[\d\.]+' | head -n 1 | sed 's/\.//g')
dl_url=https://sourceforge.net/project/hwinfo/Windows_Portable/hwi_"$get_version".zip
savename=$(basename "$dl_url")
check_version
###下载7-ZIP###
name="7-ZIP"
    echo "开始下载7-Zip"
get_version=$(curl -s https://7-zip.org/ | grep -oP 'Download 7-Zip \K\d+\.\d+' | head -n 1 | sed 's/\.//g')
dl_url=https://7-zip.org/a/7z"$get_version"-x64.exe
savename=$(basename "$dl_url")
check_version
###下载AIDA64Extreme###
name="AIDA64Extreme"
    echo "开始下载AIDA64Extreme"
get_version=$(curl -s https://www.aida64.com/downloads | grep -oP '<td class="version">\K\d+\.\d+' |head -n 1 | sed 's/\.//g')
dl_url=https://download2.aida64.com/aida64extreme"$get_version".zip
savename=$(basename "$dl_url")
check_version
#日志
log_file="/var/log/tool_updater.log"
exec > >(tee -a "$log_file") 2>&1