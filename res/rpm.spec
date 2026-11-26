Name:       polaris
Version:    1.5.0
Release:    0
Summary:    RPM package
License:    GPL-3.0
URL:        https://polaris.com
Vendor:     Polaris <support@polaris.com>
Requires:   gtk3 libxcb libXfixes alsa-lib libva2 gstreamer1-plugins-base
Recommends: libayatana-appindicator-gtk3 libxdo

# https://docs.fedoraproject.org/en-US/packaging-guidelines/Scriptlets/

%description
Polaris Remote Desktop software.

%prep
# we have no source, so nothing here

%build
# we have no source, so nothing here

%global __python %{__python3}

%install
mkdir -p %{buildroot}/usr/bin/
mkdir -p %{buildroot}/usr/share/polaris/
mkdir -p %{buildroot}/usr/share/polaris/files/
mkdir -p %{buildroot}/usr/share/icons/hicolor/256x256/apps/
mkdir -p %{buildroot}/usr/share/icons/hicolor/scalable/apps/
install -m 755 $HBB/target/release/polaris %{buildroot}/usr/bin/polaris
install $HBB/libsciter-gtk.so %{buildroot}/usr/share/polaris/libsciter-gtk.so
install $HBB/res/polaris.service %{buildroot}/usr/share/polaris/files/
install $HBB/res/128x128@2x.png %{buildroot}/usr/share/icons/hicolor/256x256/apps/polaris.png
install $HBB/res/scalable.svg %{buildroot}/usr/share/icons/hicolor/scalable/apps/polaris.svg
install $HBB/res/polaris.desktop %{buildroot}/usr/share/polaris/files/
install $HBB/res/polaris-link.desktop %{buildroot}/usr/share/polaris/files/

%files
/usr/bin/polaris
/usr/share/polaris/libsciter-gtk.so
/usr/share/polaris/files/polaris.service
/usr/share/icons/hicolor/256x256/apps/polaris.png
/usr/share/icons/hicolor/scalable/apps/polaris.svg
/usr/share/polaris/files/polaris.desktop
/usr/share/polaris/files/polaris-link.desktop
/usr/share/polaris/files/__pycache__/*

%changelog
# let's skip this for now

%pre
# can do something for centos7
case "$1" in
  1)
    # for install
  ;;
  2)
    # for upgrade
    systemctl stop polaris || true
  ;;
esac

%post
cp /usr/share/polaris/files/polaris.service /etc/systemd/system/polaris.service
cp /usr/share/polaris/files/polaris.desktop /usr/share/applications/
cp /usr/share/polaris/files/polaris-link.desktop /usr/share/applications/
systemctl daemon-reload
systemctl enable polaris
systemctl start polaris
update-desktop-database

%preun
case "$1" in
  0)
    # for uninstall
    systemctl stop polaris || true
    systemctl disable polaris || true
    rm /etc/systemd/system/polaris.service || true
  ;;
  1)
    # for upgrade
  ;;
esac

%postun
case "$1" in
  0)
    # for uninstall
    rm /usr/share/applications/polaris.desktop || true
    rm /usr/share/applications/polaris-link.desktop || true
    update-desktop-database
  ;;
  1)
    # for upgrade
  ;;
esac
