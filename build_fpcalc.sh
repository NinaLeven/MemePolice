#!/bin/bash
wget https://github.com/acoustid/chromaprint/releases/download/v1.5.1/chromaprint-fpcalc-1.5.1-linux-x86_64.tar.gz -O $HOME/fpcalc.tar.gz \
    && tar -xf $HOME/fpcalc.tar.gz -C $HOME \
    && rm $HOME/fpcalc.tar.gz
cp $HOME/chromaprint-fpcalc-1.5.1-linux-x86_64/fpcalc $HOME/fpcalc
export FPCALC_PATH=$HOME/fpcalc
export PATH=$PATH:$HOME
