FROM openeuler/openeuler:21.09
LABEL maintainer="TommyLike<tommylikehu@gmail.com>"
ARG ImageVersion="0.1.3"

# Update repo
COPY openEuler.repo /etc/yum.repos.d/openEuler.repo
RUN yum -y update && yum -y install dnf dnf-plugins-core createrepo genisoimage
# Install pip
RUN curl -fsSL https://bootstrap.pypa.io/get-pip.py -o get-pip.py && python3 get-pip.py && rm ./get-pip.py
# Install omni imager
RUN curl -L "https://github.com/omnibuildplatform/omni-imager/releases/download/v${ImageVersion}/omniimager-${ImageVersion}.tar.gz" \
    -o omniimager.tar.gz && pip3 install --prefix / omniimager.tar.gz && rm ./omniimager.tar.gz
