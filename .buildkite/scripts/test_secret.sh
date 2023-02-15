#!/bin/bash

echo "Test output with secrets - foobar"
echo "Is shown ? $MY_SECRET"

echo "Checking writing to file"
echo "${MY_SECRET}" > some_file
echo "File:"
cat some_file
echo "Is it shown using other env. var? ${OTHER_VAR}"
echo "Other vars"
echo " - FOO_VAR => ${FOO_VAR}"
echo " - TEST_VAR => ${TEST_VAR}"
