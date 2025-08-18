Promise.all([
  faceapi.nets.tinyFaceDetector.loadFromUri('https://cdn.jsdelivr.net/npm/@vladmandic/face-api/model'),
  faceapi.nets.faceLandmark68Net.loadFromUri('https://cdn.jsdelivr.net/npm/@vladmandic/face-api/model'),
]);

const imageUpload = document.getElementById('imageUpload');
const canvas = document.getElementById('canvas');
const ctx = canvas.getContext('2d');

imageUpload.addEventListener('change', async (event) => {
  const file = event.target.files[0];
  if (!file) return;

  const img = new Image();
  img.src = URL.createObjectURL(file);
  img.onload = async () => {

    canvas.width = img.width;
    canvas.height = img.height;
    ctx.drawImage(img, 0, 0, img.width, img.height);

    const options = new faceapi.TinyFaceDetectorOptions();
    const detections = await faceapi.detectAllFaces(img, options).withFaceLandmarks();

    detections.forEach(detection => {
      const { landmarks, box } = detection;

      const leftOfBoundingBox = landmarks.getRefPointsForAlignment()[0].x;
      const rightOfBoundingBox = landmarks.getRefPointsForAlignment()[1].x;
      const topOfBoundingBox = landmarks.getRefPointsForAlignment()[2].y;

      const leftEyeBrow = landmarks.getLeftEyeBrow();
      const rightEyeBrow = landmarks.getRightEyeBrow();

      const averageEyeBrowHeight = (leftEyeBrow[0].y + rightEyeBrow[0].y) / 2;
      const center = (leftOfBoundingBox + rightOfBoundingBox) / 2;

      const foreheadOffset = topOfBoundingBox - averageEyeBrowHeight;

      const foreheadTop = [
        { x: rightEyeBrow[3].x, y: averageEyeBrowHeight - foreheadOffset * .8 },
        { x: rightEyeBrow[2].x, y: averageEyeBrowHeight - foreheadOffset * .9 },
        { x: center, y: averageEyeBrowHeight - foreheadOffset },
        { x: leftEyeBrow[3].x, y: averageEyeBrowHeight - foreheadOffset * .9 },
        { x: leftEyeBrow[2].x, y: averageEyeBrowHeight - foreheadOffset * .8 },
      ];

      const faceOutline = [
        ...landmarks.getJawOutline(),
        ...foreheadTop,
      ];

      ctx.fillStyle = 'rgb(4,187,255)';
      ctx.beginPath();

      const firstPoint = faceOutline[0];
      ctx.moveTo(firstPoint.x, firstPoint.y);

      faceOutline.forEach(point => {
        ctx.lineTo(point.x, point.y);
      });

      ctx.closePath();
      ctx.fill();
      ctx.stroke();
    });
  };
});

