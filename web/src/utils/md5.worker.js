import SparkMD5 from 'spark-md5'

self.onmessage = function(e) {
  const { file, taskId } = e.data
  const chunkSize = 2 * 1024 * 1024 // 2MB chunks for hashing
  const chunks = Math.ceil(file.size / chunkSize)
  const spark = new SparkMD5.ArrayBuffer()
  let currentChunk = 0

  const reader = new FileReader()

  reader.onload = function(e) {
    spark.append(e.target.result)
    currentChunk++

    if (currentChunk < chunks) {
      loadNext()
    } else {
      const md5 = spark.end()
      self.postMessage({ taskId, md5 })
    }
  }

  reader.onerror = function(e) {
    self.postMessage({ taskId, error: 'Failed to read file' })
  }

  function loadNext() {
    const start = currentChunk * chunkSize
    const end = Math.min(start + chunkSize, file.size)
    reader.readAsArrayBuffer(file.slice(start, end))
  }

  loadNext()
}
