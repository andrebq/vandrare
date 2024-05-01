#!/usr/bin/env bash

set -eou pipefail

declare -r folder="${1}"
cd ${folder} && {
    echo -e "\n\nProcessing folder ${folder}\n\n" 1>&2
    make -Bnd | make2graph > ./make-targets.dot
    dot -Tpng < ./make-targets.dot > ./make-targets.png
}