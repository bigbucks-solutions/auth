FROM polinux/mkdocs:latest
RUN wget https://github.com/cli/cli/releases/download/v2.16.1/gh_2.16.1_linux_amd64.tar.gz
RUN tar -xvzf gh_2.16.1_linux_amd64.tar.gz
RUN mv gh_2.16.1_linux_amd64/bin/gh /usr/local/bin/gh
RUN chmod g+rwx -R /usr/local/bin/gh
RUN gh --version
RUN pip install mkdocs-material neoteroi-mkdocs mkdocs-exclude
