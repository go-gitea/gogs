for file in $(ls *.yml)
    do mv $file ${file:6}
done