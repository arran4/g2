mkdir -p /home/jules/verification/video
mkdir -p /home/jules/test_repo/profiles/default/linux/amd64/23.0/systemd
mkdir -p /home/jules/test_repo/profiles/default/linux/amd64/23.0
mkdir -p /home/jules/test_repo/profiles/base

echo "test_repo" > /home/jules/test_repo/profiles/repo_name
echo "app-misc" > /home/jules/test_repo/profiles/categories
cat << 'DESC' > /home/jules/test_repo/profiles/profiles.desc
amd64 default/linux/amd64/23.0 stable
amd64 default/linux/amd64/23.0/systemd stable
DESC

echo ".." > /home/jules/test_repo/profiles/default/linux/amd64/23.0/systemd/parent
echo "../../../base" > /home/jules/test_repo/profiles/default/linux/amd64/23.0/parent

go run cmd/g2/*.go overlays site generate -out /home/jules/test_site4 /home/jules/test_repo
