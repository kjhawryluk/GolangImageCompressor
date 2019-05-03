import math
fileNums = [1,2,3]
Ns = [2, 4, 6, 8]

with open("convolution_commands.sh", "a") as f:
	for isSequential in range(2):
		for fileNum in fileNums:
			for r in range(5):
				if isSequential == 1:
					f.write("\n(time ./editor ./csvs_for_final_project/csv_file_"+ str(fileNum) + ".csv) 2>timing_" + 
						str(isSequential) + '1' + '_' + str(fileNum) + "_" + str(r) )
				else:
					for n in Ns:
						f.write("\n(time ./editor ./csvs_for_final_project/csv_file_"+ str(fileNum) + ".csv p="+ str(n) +") 2>timing_"  + 
						str(isSequential) + str(n) + '_' + str(fileNum) + "_" +  str(r))


